package servers

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/murdinc/cli"
	"github.com/murdinc/crusher/specr"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Represents a single remote server
type Server struct {
	Name     string `ini:"-"` // considered Sections in config file
	Host     string
	Username string
	Spec     string
	PassAuth bool
	Password string `ini:"-"` // Not stored in config, just where it gets temporarily stored when we ask for it.
}

// Slice of remote servers with attached methods
type Servers []Server

// Remote Job
type RemoteJob struct {
	net.Conn
	Server    Server
	SSHConf   *ssh.ClientConfig
	Timeout   time.Duration
	Responses chan string
	Errors    chan error
	WaitGroup *sync.WaitGroup
	SpecList  *specr.SpecList
	SpecName  string
}

// Assembles a new Server struct
func New(name, host, username, spec string, passAuth bool) *Server {
	// Maybe do sanity checking here until a function with a callback is added to the cli library?
	server := new(Server)

	server.Name = name
	server.Host = host
	server.Username = username
	server.Spec = spec
	server.PassAuth = passAuth

	return server

}

// Prints a single server config data in a table
func (s *Server) PrintServerInfo() {

	collumns := []string{"Name", "Host", "Username", "Spec", "Password Auth?"}
	var rows [][]string

	rows = append(rows, []string{
		s.Name,
		s.Host,
		s.Username,
		s.Spec,
		fmt.Sprintf("%t", s.PassAuth),
	})

	printTable(collumns, rows)
}

// Read function with timeout
func (r *RemoteJob) Read(b []byte) (int, error) {
	err := r.Conn.SetReadDeadline(time.Now().Add(r.Timeout))
	if err != nil {
		return 0, err
	}
	return r.Conn.Read(b)
}

// Write function with timeout
func (r *RemoteJob) Write(b []byte) (int, error) {
	err := r.Conn.SetWriteDeadline(time.Now().Add(r.Timeout))
	if err != nil {
		return 0, err
	}
	return r.Conn.Write(b)
}

// Run Remote Configuration on a target spec group
func (s Servers) RemoteConfigure(specName string, specList *specr.SpecList) {

	// Get our list of targets
	targetGroup := s.getTargetGroup(specName)

	configure := cli.PromptBool("Do you want to configure these servers?")

	if !configure {
		cli.Information("Okay, maybe next time..")
		os.Exit(0)
	}

	// Get passwords for hosts that need them
	for i, server := range targetGroup {
		if server.PassAuth {
			targetGroup[i].Password = cli.PromptPassword(fmt.Sprintf("Please enter your password for user [%s] on remote server [%s]:", server.Username, server.Host))
		}
	}

	cli.Information("Great! I'll make it so..")

	responses := make(chan string, 10)
	errors := make(chan error, 10)

	// hold onto your butts
	var wg sync.WaitGroup
	wg.Add(len(targetGroup))

	for _, server := range targetGroup {

		sshConf := &ssh.ClientConfig{
			User: server.Username,
		}

		if server.PassAuth {
			sshConf.Auth = []ssh.AuthMethod{ssh.Password(server.Password)}
		} else {
			cli.Information("SSH Key Auth is not yet implemented")
			wg.Done()
			continue
			//sshConf.Auth =ssh.ClientAuth{ .. ssh stuff .. }
		}

		timeout := time.Second * 7
		job := RemoteJob{
			Server:    server,
			Responses: responses,
			Errors:    errors,
			Timeout:   timeout,
			SSHConf:   sshConf,
			WaitGroup: &wg,
			SpecList:  specList,
			SpecName:  specName}

		// Launch it!
		go job.Run()

	}

	// Display Output of Jobs
	go func() {
		for {
			select {
			case resp := <-responses:
				printResp(resp)
			case err := <-errors:
				printErr(err.Error())
			}
		}
	}()

	wg.Wait()

	time.Sleep(time.Second)
}

func printResp(msg string) {
	template := `{{ ansi "fggreen"}}✓  {{ . }}{{ansi ""}}
	`
	cli.PrintAnsi(template, msg)
}

func printErr(msg string) {
	template := `{{ ansi "fgred"}}X  {{ . }}{{ansi ""}}
	`
	cli.PrintAnsi(template, msg)
}

// Runs the remote Jobs and returns results on the job channels
func (job *RemoteJob) Run() {

	// Setup
	////////////////..........

	defer job.WaitGroup.Done()
	header := addSpaces(fmt.Sprintf("[%s - %s]", job.Server.Name, job.Server.Host), 35)

	// Open a tcp connection with a timeout
	conn, err := net.DialTimeout("tcp", job.Server.Host+":22", job.Timeout)

	if err != nil {
		job.Errors <- fmt.Errorf("%s DialTimeout Failed: %s", header, err)
		return
	}
	job.Conn = conn // so that it gets wrapped with our timeout funcs

	// Get an ssh client
	c, chans, reqs, err := ssh.NewClientConn(job.Conn, job.Server.Host, job.SSHConf)
	if err != nil {
		job.Errors <- fmt.Errorf("%s NewClientConn Failed: %s", header, err)
		return
	}
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()

	// Actual Work
	////////////////..........

	// Run Apt-Get Commands
	aptCmd := job.SpecList.AptGetCmd(job.SpecName)
	job.Responses <- fmt.Sprintf("%s Running apt-get Command... \n		Command: [%s]\n", header, aptCmd)
	_, err = runCommand(client, aptCmd)
	if err != nil {
		job.Errors <- fmt.Errorf("%s apt-get Command Failed! \n		Command: [%s]\n		Error: %s", header, aptCmd, err)
	} else {
		job.Responses <- fmt.Sprintf("%s apt-get Command Finished!", header) // TODO verbose output resp?
	}

	// Transfer any files we need to transfer
	fileList := job.SpecList.DebianFileTransferList(job.SpecName)
	_, err = transferFiles(client, fileList)

	// Run post configure commands
	postCmd := job.SpecList.PostCmd(job.SpecName)
	job.Responses <- fmt.Sprintf("%s Running Post Configuration Command... \n		Command: [%s]\n", header, aptCmd)
	_, err = runCommand(client, postCmd)
	if err != nil {
		job.Errors <- fmt.Errorf("%s Post Configure Command Failed! \n		Command: [%s]\n		Error: %s", header, postCmd, err)
	} else {
		job.Responses <- fmt.Sprintf("%s Post Configure Command Finished!", header)
	}

	// End of the line
}

func runCommand(client *ssh.Client, cmd string) (string, error) {
	// Open an ssh session
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf

	err = session.Run(cmd)

	return stdoutBuf.String(), err

}

func transferFiles(client *ssh.Client, fileList *specr.FileTransfers) (string, error) {
	// open an sftp session.
	sftp, err := sftp.NewClient(client)
	if err != nil {
		return "", err
	}
	defer sftp.Close()

	return "", nil

}

// Prints all server config data in a table
func (servers Servers) PrintAllServerInfo() {

	// Build the table elements
	collumns := []string{"#", "Name", "Host", "Username", "Spec", "Password Auth?"}

	var rows [][]string

	for i, s := range servers {
		rows = append(rows, []string{
			fmt.Sprint(i + 1),
			s.Name,
			s.Host,
			s.Username,
			s.Spec,
			fmt.Sprintf("%t", s.PassAuth),
		})
	}

	printTable(collumns, rows)
}

// Gets the target group of servers for a specified spec
func (servers Servers) getTargetGroup(spec string) Servers {

	collumns := []string{"Name", "Host", "Username", "Spec", "Password Auth?"}
	var rows [][]string
	var targetGroup Servers

	for _, s := range servers {
		if s.Spec == spec {

			rows = append(rows, []string{
				s.Name,
				s.Host,
				s.Username,
				s.Spec,
				fmt.Sprintf("%t", s.PassAuth),
			})

			targetGroup = append(targetGroup, s)
		}
	}

	if len(rows) == 0 {
		cli.Information(fmt.Sprintf("I couldn't find any servers set up with the spec [%s], here is what I do have: ", spec))
		servers.PrintAllServerInfo()
		os.Exit(0)
	}

	cli.Information(fmt.Sprintf("I found the following servers with the spec [%s]:", spec))
	printTable(collumns, rows)

	return targetGroup

}

// Table helper
func printTable(collumns []string, rows [][]string) {
	fmt.Println("")
	t := cli.NewTable(rows, &cli.TableOptions{
		Padding:      1,
		UseSeparator: true,
	})
	t.SetHeader(collumns)
	fmt.Println(t.Render())
}

func addSpaces(s string, w int) string {
	if len(s) < w {
		s += strings.Repeat(" ", w-len(s))
	}
	return s
}
