package specr

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/murdinc/cli"

	"gopkg.in/ini.v1"
)

// Spec Structs
////////////////..........
type SpecList struct {
	Specs map[string]*Spec
	// using a map for fast lookups, but maybe we want to use a slice if we start caring about the order they output
}

type Spec struct {
	Version  string   `ini:"VERSION"`
	Requires []string `ini:"REQUIRES"`
	Packages Packages `ini:"PACKAGES"`
	Configs  Configs  `ini:"CONFIGS"`
	Content  Content  `ini:"CONTENT"`
	Commands Commands `ini:"COMMANDS"`
	SpecFile string   `ini:"-"`
	SpecRoot string   `ini:"-"`
}

type Packages struct {
	AptGet []string `ini:"apt_get"`
}

type Configs struct {
	DebianRoot string `ini:"debian_root"`
}

type Content struct {
	Source     string `ini:"source"`
	DebianRoot string `ini:"debian_root"`
}

type Commands struct {
	Post string `ini:"post"`
}

// FileTransfer Struct
////////////////..........
type FileTransfer struct {
	Source      string
	Destination string
	Folder      string
	Chown       string
	Chmod       string
}

// Jobs that run locally
type LocalJob struct {
	Responses chan string
	Errors    chan error
	SpecName  string
	SpecList  *SpecList
	WaitGroup *sync.WaitGroup
}

type FileTransfers []FileTransfer

// Reads in all the specs and builds a SpecList
func GetSpecs() (*SpecList, error) {

	var err error
	specList := new(SpecList)
	specList.Specs = make(map[string]*Spec)

	currentUser, _ := user.Current()
	candidates := []string{
		os.Getenv("GOPATH") + "/src/github.com/murdinc/crusher/example-specs/",
		"/etc/crusher/specs/",
		"./specs/",
		currentUser.HomeDir + "/crusher/specs/",
	}

	walkFn := func(path string, fileInfo os.FileInfo, inErr error) (err error) {
		//fmt.Printf("%v\n", fileInfo.Name())
		if inErr == nil && !fileInfo.IsDir() && strings.HasSuffix(strings.ToLower(fileInfo.Name()), ".spec") {
			err = specList.scanFile(path)
		}
		return
	}

	// Walk each of the candidate folders
	for _, folder := range candidates {
		err = filepath.Walk(folder, walkFn)
	}

	return specList, err
}

// Scans a given file and if it is a spec, adds it to the spec list
func (s *SpecList) scanFile(file string) error {
	// This is most likely a spec file, so lets try to pull a struct from it

	cfg, err := ini.Load(file)
	if err != nil {
		return err
	}

	specName := cfg.Section("").Key("NAME").String()

	spec := new(Spec)

	if len(specName) > 0 {
		err := cfg.MapTo(spec)
		if err != nil {
			return err
		}
		spec.SpecFile = file
		spec.SpecRoot = path.Dir(file)
		s.Specs[specName] = spec
	}

	return nil
}

// Checks if a given spec exists
func (s *SpecList) SpecExists(spec string) bool {
	if _, ok := s.Specs[spec]; ok {
		return true
	}
	return false
}

// Returns the apt-get command for a given spec
func (s *SpecList) AptGetCmd(specName string) string {

	packages := s.getAptPackages(specName)
	cmd := "sudo apt-get update && sudo apt-get install -y " + strings.Join(packages, " ") // TODO less hard coded?

	return cmd
}

// Returns the post-configure command
func (s *SpecList) PostCmd(specName string) string {

	postConf := s.getPostCommands(specName)
	cmd := strings.Join(postConf, " && ")

	return cmd
}

func (s *SpecList) DebianFileTransferList(specName string) *FileTransfers {

	fileTransfers := new(FileTransfers)

	fileTransfers = s.getDebianFileTransfers(specName)
	return fileTransfers

}

// Recursive unexported func for FileTransferList
func (s *SpecList) getDebianFileTransfers(specName string) *FileTransfers {
	// much append, sry. TODO?
	// TODO split up?

	// The requested spec
	spec := s.Specs[specName]

	files := new(FileTransfers)

	// Spec Configs
	////////////////..........
	srcConfFolder := spec.SpecRoot + "/configs/"
	destConfFolder := spec.Configs.DebianRoot

	if spec.Configs.DebianRoot != "" {
		// Walk the Configs folder and append each file
		walkFn := func(path string, fileInfo os.FileInfo, inErr error) (err error) {
			if inErr == nil && !fileInfo.IsDir() {
				destination := destConfFolder + strings.TrimPrefix(path, srcConfFolder)
				files.add(FileTransfer{
					Source:      path,
					Destination: destination,
					Folder:      filepath.Dir(destination),
				})
			}
			return
		}
		filepath.Walk(srcConfFolder, walkFn)
	}

	// Spec Content
	////////////////..........
	srcContentFolder := spec.SpecRoot + "/content/"
	destContentFolder := spec.Content.DebianRoot

	if spec.Content.DebianRoot != "" && spec.Content.Source == "spec" {
		// Walk the Configs folder and append each file
		walkFn := func(path string, fileInfo os.FileInfo, inErr error) (err error) {
			if inErr == nil && !fileInfo.IsDir() {
				destination := destContentFolder + strings.TrimPrefix(path, srcContentFolder)
				files.add(FileTransfer{
					Source:      path,
					Destination: destination,
					Folder:      filepath.Dir(destination),
				})
			}
			return
		}
		filepath.Walk(srcContentFolder, walkFn)
	}

	// Requirement Spec File List
	////////////////..........
	for _, reqSpec := range spec.Requires {
		recFiles := s.getDebianFileTransfers(reqSpec)
		*files = append(*files, *recFiles...) // dereference blowout sale!
	}

	return files
}

func (f *FileTransfers) add(file FileTransfer) {
	*f = append(*f, file)
}

func (s *SpecList) ShowSpec(specName string) {
	cli.Information(fmt.Sprintf("[APT-GET COMMAND] >$ %s", s.AptGetCmd(specName)))
	cli.Information("File Transfer List:")

	fileList := s.DebianFileTransferList(specName)

	for i, file := range *fileList {
		fmt.Printf("#%d - \n		Source: %s \n		Destination: %s\n		Folder: %s\n\n", i+1, file.Source, file.Destination, file.Folder)
	}

	cli.Information(fmt.Sprintf("[POST CONFIGURE COMMAND] >$ %s", s.PostCmd(specName)))

}

// Run Local configuration on this machine
func (s *SpecList) LocalConfigure(specName string) {
	// Doesn't really need to use a goroutine now, but maybe we want to run tasks concurrently in the future?

	var wg sync.WaitGroup
	wg.Add(1)

	responses := make(chan string, 10)
	errors := make(chan error, 10)

	job := LocalJob{
		Responses: responses,
		Errors:    errors,
		SpecName:  specName,
		SpecList:  s,
		WaitGroup: &wg}

	// Launch it!
	go job.Run()

	// Display Output of Job
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

func (job *LocalJob) Run() {
	defer job.WaitGroup.Done()

	line := addSpaces("[%s] [local-configure]", 45) + " >> %s " // status, message

	// Run Apt-Get Commands
	aptCmd := job.SpecList.AptGetCmd(job.SpecName)
	job.Responses <- fmt.Sprintf(line, "*", "Running apt-get Command...")
	err := job.runCommand(aptCmd, "apt-get")
	if err != nil {
		job.Errors <- fmt.Errorf(line, "X", "Command apt-get Failed! Aborting futher tasks for this server..")
		return
	}
	job.Responses <- fmt.Sprintf(line, "✓", "Command apt-get Succeeded!")

	// Transfer any files we need to transfer
	fileList := job.SpecList.DebianFileTransferList(job.SpecName)
	job.Responses <- fmt.Sprintf(line, "*", "Starting file copy...")
	err = job.transferFiles(fileList, "Configuration and Content Files")
	if err != nil {
		job.Errors <- fmt.Errorf(line, "X", "File Copy Failed! Aborting futher tasks for this server..")
		return
	}
	job.Responses <- fmt.Sprintf(line, "✓", "File Copt Succeeded!")

	// Run post configure commands
	postCmd := job.SpecList.PostCmd(job.SpecName)
	job.Responses <- fmt.Sprintf(line, "*", "Running Post-Configuration Command...")
	err = job.runCommand(postCmd, "Post-Configuration")
	if err != nil {
		job.Errors <- fmt.Errorf(line, "X", "Post-Configuration Command Failed!")
	}
	job.Responses <- fmt.Sprintf(line, "✓", "Post-Configuration Command Succeeded!")

	// End of the line
}

func (j *LocalJob) runCommand(command string, name string) error {

	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], parts[1:]...)

	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf

	err := cmd.Run()

	//j.Responses <- stdoutBuf.String() // TODO handle more verbose output, maybe from a verbose cli flag

	return err

}

func (j *LocalJob) transferFiles(fileList *FileTransfers, name string) error {

	line := addSpaces("[%s] [local-configure]", 45) + " >> %s " // status, message

	// Defer cleanup
	defer j.runCommand("sudo rm -rf /tmp/crusher/*", "")

	for _, file := range *fileList {

		// Make our temp folder
		j.runCommand("mkdir -p /tmp/crusher/"+file.Folder, "")
		err := j.runCommand("sudo mkdir -p "+file.Folder, "") // should prob add chown and chmod to the config structs to set it afterwards
		if err != nil {
			j.Errors <- fmt.Errorf(line, "X", "Unable to make directory: "+file.Folder)
			return err
		}

		j.Responses <- fmt.Sprintf(line, "*", "Copying file: "+file.Destination)

		// Read the file
		////////////////..........
		rf, err := os.Open(file.Source)
		defer rf.Close()
		if err != nil {
			j.Errors <- fmt.Errorf(line, "X", "Unable to open file: "+file.Source)
			return err
		}

		rfi, err := rf.Stat()
		if err != nil {
			j.Errors <- fmt.Errorf(line, "X", "Unable to inspect file: "+file.Source)
			return err
		}

		fileSize := rfi.Size()
		fileBytes := make([]byte, fileSize)

		_, err = rf.Read(fileBytes)
		if err != nil {
			j.Errors <- fmt.Errorf(line, "X", "Unable to read file: "+file.Source)
			return err
		}

		// Write the file
		////////////////..........
		rf, err = os.Create("/tmp/crusher" + file.Destination)
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

		j.Responses <- fmt.Sprintf(line, "✓", "Completed copy of file: "+file.Destination)
	}

	return nil

}

func printResp(msg string) {
	template := `{{ ansi "fggreen"}}{{ . }}{{ansi ""}}
	`
	cli.PrintAnsi(template, msg)
}

func printErr(msg string) {
	template := `{{ ansi "fgred"}}{{ . }}{{ansi ""}}
	`
	cli.PrintAnsi(template, msg)
}

// Prints table of all available specs in a table
func (s *SpecList) PrintSpecTable() {

	// Build the table elements
	collumns := []string{"Spec Name", "Version", "Requires", "Apt Packages", "Debian Config Root", "Content Source", "Debian Content Root", "Post Commands", "Spec File"}

	var rows [][]string

	// TODO this is a wide table, so when values get longer we will need to trim some stuff.
	for name, spec := range s.Specs {
		rows = append(rows, []string{
			name,
			spec.Version,
			strings.Join(spec.Requires, ", "),
			strings.Join(spec.Packages.AptGet, ", "),
			spec.Configs.DebianRoot,
			spec.Content.Source,
			spec.Content.DebianRoot,
			spec.Commands.Post,
			strings.TrimPrefix(spec.SpecFile, os.Getenv("GOPATH")),
		})
	}

	printTable(collumns, rows)

}

// Recursive unexported func for PostCmd
func (s *SpecList) getPostCommands(specName string) []string {
	// The requested spec
	spec := s.Specs[specName]
	var commands []string

	// gather all required post configure commands for this spec
	if spec.Commands.Post != "" {
		commands = append(commands, spec.Commands.Post)
	}

	// Loop through this specs requirements to all other post configure commands we need
	for _, reqSpec := range spec.Requires {
		commands = append(commands, s.getPostCommands(reqSpec)...)
	}

	return commands
}

// Recursive unexported func for AptGetCmd
func (s *SpecList) getAptPackages(specName string) []string {
	// The requested spec
	spec := s.Specs[specName]
	var packages []string

	// gather all required apt-get packages for this spec
	packages = append(packages, spec.Packages.AptGet...)

	// Loop through this specs requirements gather to all other apt-get packages we need
	for _, reqSpec := range spec.Requires {
		packages = append(packages, s.getAptPackages(reqSpec)...)
	}

	return packages
}

// Table helper
// Used these twice in one project already, maybe its time to move it?
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
