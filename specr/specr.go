package specr

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/murdinc/cli"

	"gopkg.in/ini.v1"
)

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
	SpecFile string
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

// Reads in all the specs and builds a SpecList
func GetSpecs() (*SpecList, error) {

	var err error
	specList := new(SpecList)
	specList.Specs = make(map[string]*Spec)

	currentUser, _ := user.Current()
	candidates := []string{
		currentUser.HomeDir + "/.crusher_specs/",
		currentUser.HomeDir + "/.crusher_specs/",
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

	for _, folder := range candidates {
		// Walk each of the candidates
		err = filepath.Walk(folder, walkFn)
	}

	return specList, err
}

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
		s.Specs[specName] = spec
	}

	return nil

}

// Prints all available specs in a table
func (s *SpecList) PrintAllSpecs() {

	// Build the table elements
	collumns := []string{"Name", "Version", "Requires", "Apt Packages", "Debian Config Root", "Content Source", "Content Debian Root", "Post Commands", "Spec File"}

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
