package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"path"
	"text/template"

	"github.com/pschlump/sprig"
	"google.golang.org/protobuf/compiler/protogen"
)

//go:embed server_tmpl/mock_server.tmpl
var MockServerTemplate string

//go:embed cmd_tmpl/server.tmpl
var CmdServerTemplate string

//go:embed cmd_tmpl/Makefile.tmpl
var CmdMakefileTemplate string

//go:embed cmd_tmpl/Dockerfile.tmpl
var CmdDockerfileTemplate string

func log(msg string, args ...interface{}) {
	os.Stderr.WriteString(fmt.Sprintf(msg+"\n", args))
}

func generateFileAndExecuteTemplate(plugin *protogen.Plugin, goImportPath protogen.GoImportPath, filename string, tmplText string, templateData any) error {
	generatedFile := plugin.NewGeneratedFile(filename, goImportPath)
	// Adding qualifiedIdent function to the template. This will allow using an imported message in case the input/output
	// is from another proto message packcage
	qualifiedIdent := func(goIdent protogen.GoIdent) string {
		return generatedFile.QualifiedGoIdent(goIdent)
	}
	qualifiedIdentCustom := func(importPath protogen.GoImportPath, name string) string {
		goIdent := protogen.GoIdent{
			GoName:       name,
			GoImportPath: importPath,
		}
		return generatedFile.QualifiedGoIdent(goIdent)
	}

	tmpl := template.Must(template.New(filename).Funcs(template.FuncMap{
		"qualifiedIdent":       qualifiedIdent,
		"qualifiedIdentCustom": qualifiedIdentCustom,
	}).Funcs(sprig.TxtFuncMap()).Option("missingkey=error").Parse(tmplText))
	if err := tmpl.Execute(generatedFile, templateData); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}
	return nil
}

func generateCmds(plugin *protogen.Plugin) error {
	cmdsDirectory := "grpcmock_cmds"

	if err := generateFileAndExecuteTemplate(plugin, "", path.Join(cmdsDirectory, "server.go"), CmdServerTemplate, plugin); err != nil {
		return fmt.Errorf("generate cmd server: %w", err)
	}

	if err := generateFileAndExecuteTemplate(plugin, "", path.Join(cmdsDirectory, "Dockerfile"), CmdDockerfileTemplate, plugin); err != nil {
		return fmt.Errorf("generate cmd Dockerfile: %w", err)
	}

	if err := generateFileAndExecuteTemplate(plugin, "", path.Join(cmdsDirectory, "Makefile"), CmdMakefileTemplate, plugin); err != nil {
		return fmt.Errorf("generate cmd Makefile: %w", err)
	}

	return nil
}

func generateFile(plugin *protogen.Plugin, f *protogen.File) error {
	if len(f.Services) == 0 {
		return nil
	}

	filename := f.GeneratedFilenamePrefix + "_grpcmock.pb.go"
	return generateFileAndExecuteTemplate(plugin, f.GoImportPath, filename, MockServerTemplate, f)
}

func main() {
	var flags flag.FlagSet
	shouldGenerateCmds := flags.Bool("generate-cmds", true, "generate cmds main packages for each mocked service")
	generated := false

	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(plugin *protogen.Plugin) error {
		for _, f := range plugin.Files {
			if !f.Generate {
				continue
			}

			if err := generateFile(plugin, f); err != nil {
				return fmt.Errorf("generate file %q: %w", f.GeneratedFilenamePrefix, err)
			}
			generated = true
		}

		if !generated {
			return nil
		}

		if *shouldGenerateCmds {
			if err := generateCmds(plugin); err != nil {
				return fmt.Errorf("getnerate cmds: %w", err)
			}
		}
		return nil
	})
}
