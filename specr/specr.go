package specr

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

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
	Chown       string
	Chmod       string
}

type FileTransfers []FileTransfer

// Reads in all the specs and builds a SpecList
func GetSpecs() (*SpecList, error) {

	var err error
	specList := new(SpecList)
	specList.Specs = make(map[string]*Spec)

	currentUser, _ := user.Current()
	candidates := []string{
		currentUser.HomeDir + "/crusher_specs/",
		currentUser.HomeDir + "/crusher-example-specs/",
		"/etc/crusher/specs/",
		"./example-specs/",
	}

	//
	walkFn := func(path string, fileInfo os.FileInfo, inErr error) (err error) {
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

	// Configs
	////////////////..........
	srcConfFolder := spec.SpecRoot + "/configs/"
	destConfFolder := spec.Configs.DebianRoot

	if spec.Configs.DebianRoot != "" {
		// Walk the Configs folder and append each file
		walkFn := func(path string, fileInfo os.FileInfo, inErr error) (err error) {
			if inErr == nil && !fileInfo.IsDir() {
				files.add(FileTransfer{
					Source:      path,
					Destination: destConfFolder + strings.TrimPrefix(path, srcConfFolder),
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
				files.add(FileTransfer{
					Source:      path,
					Destination: destContentFolder + strings.TrimPrefix(path, srcContentFolder),
				})
			}
			return
		}
		filepath.Walk(srcContentFolder, walkFn)
	}

	// Recursive Spec Requirements
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
		fmt.Printf("#%d - \n		Source: %s \n		Destination: %s\n\n", i+1, file.Source, file.Destination)
	}

	cli.Information(fmt.Sprintf("[POST CONFIGURE COMMAND] >$ %s", s.PostCmd(specName)))

}

// Prints table of all available specs in a table
func (s *SpecList) PrintSpecTable() {

	// Build the table elements
	collumns := []string{"Spec Name", "Version", "Requires", "Apt Packages", "Debian Config Root", "Content Source", "Content Debian Root", "Post Commands", "Spec File"}

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
			spec.SpecFile,
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
// Used this twice in one project already, maybe its time to move it?
func printTable(collumns []string, rows [][]string) {
	fmt.Println("")
	t := cli.NewTable(rows, &cli.TableOptions{
		Padding:      1,
		UseSeparator: true,
	})
	t.SetHeader(collumns)
	fmt.Println(t.Render())
}
