package servers

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/murdinc/crusher/specr"
	"github.com/murdinc/terminal"
	"github.com/olekukonko/tablewriter"
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
	Client    *ssh.Client
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
func (s Servers) RemoteConfigure(search string, specList *specr.SpecList) {

	// Get our list of targets
	targetGroup := s.getTargetGroup(search)

	configure := terminal.PromptBool("Do you want to configure these servers?")

	if !configure {
		terminal.Information("Okay, maybe next time..")
		return
	}

	// Get passwords for hosts that need them
	for i, server := range targetGroup {
		if server.PassAuth {
			targetGroup[i].Password = terminal.PromptPassword(fmt.Sprintf("Please enter your password for user [%s] on remote server [%s]:", server.Username, server.Host))
		}
	}

	terminal.Information("Great! I'll make it so..")

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
			terminal.Information("SSH Key Auth is not yet implemented")
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
			SpecName:  server.Spec}

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
	template := `{{ ansi "fggreen"}}{{ . }}{{ansi ""}}
	`
	terminal.PrintAnsi(template, msg)
}

func printErr(msg string) {
	template := `{{ ansi "fgred"}}{{ . }}{{ansi ""}}
	`
	terminal.PrintAnsi(template, msg)
}

// Runs the remote Jobs and returns results on the job channels
func (job *RemoteJob) Run() {

	line := addSpaces("[%s] ["+job.Server.Name+" - "+job.Server.Host+"]", 45) + " >> %s " // status, name, host, message

	// Setup
	////////////////..........

	defer job.WaitGroup.Done()

	// Open a tcp connection with a timeout
	job.Responses <- fmt.Sprintf(line, "*", "Opening a new TCP connection...")
	conn, err := net.DialTimeout("tcp", job.Server.Host+":22", job.Timeout)

	if err != nil {
		job.Errors <- fmt.Errorf(line, "X", "Unable to open TCP connection! Aborting futher tasks for this server..")
		return
	}
	job.Conn = conn // so that it gets wrapped with our timeout funcs
	job.Responses <- fmt.Sprintf(line, "✓", "TCP connection Opened!")

	// Get an ssh client
	job.Responses <- fmt.Sprintf(line, "*", "Creating new ssh client...")
	c, chans, reqs, err := ssh.NewClientConn(job.Conn, job.Server.Host, job.SSHConf)
	if err != nil {
		job.Errors <- fmt.Errorf(line, "X", "Unable to create SSH client! Aborting futher tasks for this server..")
		return
	}
	job.Client = ssh.NewClient(c, chans, reqs)
	defer job.Client.Close()
	job.Responses <- fmt.Sprintf(line, "✓", "SSH client creation Succeeded!")

	// Elevate permissions
	job.Responses <- fmt.Sprintf(line, "*", "Attempting to elevate permissions...")
	err = job.runCommand("sudo uname", "sudo uname")
	if err != nil {
		job.Errors <- fmt.Errorf(line, "X", "Permission Elevation Failed! Aborting futher tasks for this server..")
		return
	}
	job.Responses <- fmt.Sprintf(line, "✓", "Permission Elevation Succeeded!")

	// Actual Work
	////////////////..........

	// Run pre configure commands
	preCmds := job.SpecList.PreCmds(job.SpecName)
	for _, preCmd := range preCmds {
		job.Responses <- fmt.Sprintf(line, "*", "Running Pre-Configuration Command...")
		err = job.runCommand(preCmd, "Pre-Configuration")
		if err != nil {
			job.Errors <- fmt.Errorf(line, "X", "Pre-Configuration Command Failed! Aborting futher tasks for this server..")
			job.Errors <- fmt.Errorf("Error: %s", err)
			return
		}
		job.Responses <- fmt.Sprintf(line, "✓", "Pre-Configuration Command Succeeded!")
	}

	// Run Apt-Get Commands
	aptCmds := job.SpecList.AptGetCmds(job.SpecName)
	job.Responses <- fmt.Sprintf(line, "*", "Running apt-get Command...")
	for _, aptCmd := range aptCmds {
		err = job.runCommand(aptCmd, "apt-get")
		if err != nil {
			job.Errors <- fmt.Errorf(line, "X", "Command apt-get Failed! Aborting futher tasks for this server..")
			return
		}
		job.Responses <- fmt.Sprintf(line, "✓", "Command apt-get Succeeded!")
	}

	// Transfer any files we need to transfer
	fileList := job.SpecList.DebianFileTransferList(job.SpecName)
	job.Responses <- fmt.Sprintf(line, "*", "Starting remote file transfer...")
	err = job.transferFiles(fileList, "Configuration and Content Files")
	if err != nil {
		job.Errors <- fmt.Errorf(line, "X", "File Transfer Failed! Aborting futher tasks for this server..")
		return
	}
	job.Responses <- fmt.Sprintf(line, "✓", "File Transfer Succeeded!")

	// Run post configure commands
	postCmds := job.SpecList.PostCmds(job.SpecName)
	for _, postCmd := range postCmds {
		job.Responses <- fmt.Sprintf(line, "*", "Running Post-Configuration Command...")
		err = job.runCommand(postCmd, "Post-Configuration")
		if err != nil {
			job.Errors <- fmt.Errorf(line, "X", "Post-Configuration Command Failed!")
		}
		job.Responses <- fmt.Sprintf(line, "✓", "Post-Configuration Command Succeeded!")
	}

	// End of the line
}

func (j *RemoteJob) runCommand(cmd string, name string) error {

	// Open an ssh session
	session, err := j.Client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf

	err = session.Run(cmd)

	//j.Responses <- stdoutBuf.String() // TODO handle more verbose output, maybe from a verbose cli flag

	return err

}

func (j *RemoteJob) transferFiles(fileList *specr.FileTransfers, name string) error {

	line := addSpaces("[%s] ["+j.Server.Name+" - "+j.Server.Host+"]", 45) + " >> %s " // status, name, host, message

	// open an sftp session.
	sftpClient, err := sftp.NewClient(j.Client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	// Defer cleanup
	defer j.runCommand("sudo rm -rf /tmp/crusher/*", "")

	for _, file := range *fileList {

		// Make our temp folder
		j.runCommand("mkdir -p /tmp/crusher/"+file.Folder, "")
		err = j.runCommand("sudo mkdir -p "+file.Folder, "") // should prob add chown and chmod to the config structs to set it afterwards
		if err != nil {
			j.Errors <- fmt.Errorf(line, "X", "Unable to make directory: "+file.Folder)
			return err
		}

		j.Responses <- fmt.Sprintf(line, "*", "Uploading file: "+file.Destination)

		// Read the local file
		////////////////..........
		lf, err := os.Open(file.Source)
		defer lf.Close()
		if err != nil {
			j.Errors <- fmt.Errorf(line, "X", "Unable to open local file: "+file.Source)
			return err
		}

		lfi, err := lf.Stat()
		if err != nil {
			j.Errors <- fmt.Errorf(line, "X", "Unable to inspect local file: "+file.Source)
			return err
		}

		fileSize := lfi.Size()
		fileBytes := make([]byte, fileSize)

		_, err = lf.Read(fileBytes)
		if err != nil {
			j.Errors <- fmt.Errorf(line, "X", "Unable to read local file: "+file.Source)
			return err
		}

		// Write the remote file
		////////////////..........
		rf, err := sftpClient.Create("/tmp/crusher" + file.Destination)
		defer rf.Close()

		if err != nil {
			j.Errors <- fmt.Errorf(line, "X", "Unable to create file: "+file.Destination)
			return err
		}
		if _, err := rf.Write(fileBytes); err != nil {
			j.Errors <- fmt.Errorf(line, "X", "Unable to write file: "+file.Destination)
			return err
		}

		// mv
		j.runCommand("sudo mv /tmp/crusher"+file.Destination+" "+file.Destination, "")

		j.Responses <- fmt.Sprintf(line, "✓", "Completed upload of file: "+file.Destination)
	}

	return nil

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
func (servers Servers) getTargetGroup(search string) Servers {

	collumns := []string{"Name", "Host", "Username", "Spec", "Password Auth?"}
	var rows [][]string
	var targetGroup Servers

	for _, s := range servers {
		if s.Spec == search || s.Name == search {

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
		terminal.Information(fmt.Sprintf("I couldn't find any servers with the name or spec of: [%s], here is what I do have: ", search))
		servers.PrintAllServerInfo()
		os.Exit(0)
	}

	terminal.Information(fmt.Sprintf("I found the following servers under [%s]:", search))
	printTable(collumns, rows)

	return targetGroup

}

// Table helper
func printTable(header []string, rows [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.AppendBulk(rows)
	table.Render()
}

func addSpaces(s string, w int) string {
	if len(s) < w {
		s += strings.Repeat(" ", w-len(s))
	}
	return s
}
