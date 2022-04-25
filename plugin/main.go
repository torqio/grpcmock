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

//go:embed cmd_tmpl/registry.tmpl
var RegistryTemplate string

func log(msg string, args ...interface{}) {
	os.Stderr.WriteString(fmt.Sprintf(msg+"\n", args...))
}

func generateFileAndExecuteTemplate(plugin *protogen.Plugin, goImportPath protogen.GoImportPath, manualImports []string, filename string, tmplText string, templateData any) error {
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

	for _, manualImport := range manualImports {
		// This will make protogen to import this package (without '_' prefix)
		qualifiedIdentCustom(protogen.GoImportPath(manualImport), "")
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

func findBasePath(plugin *protogen.Plugin) (string, error) {
	for _, f := range plugin.Files {
		if !f.Generate {
			continue
		}
		if len(f.Services) == 0 {
			continue
		}
		return path.Dir(f.GeneratedFilenamePrefix), nil
	}
	return "", fmt.Errorf("no suitable file found for getting base path")
}

func generateCmds(plugin *protogen.Plugin) error {
	basePath, err := findBasePath(plugin)
	if err != nil {
		return err
	}
	cmdsDirectory := path.Join(basePath, "grpcmock_cmds")

	if err = generateFileAndExecuteTemplate(plugin, "", nil, path.Join(cmdsDirectory, "server.go"), CmdServerTemplate, plugin); err != nil {
		return fmt.Errorf("generate cmd server: %w", err)
	}

	for _, f := range plugin.Files {
		if !f.Generate {
			continue
		}
		if len(f.Services) == 0 {
			continue
		}
		baseName := path.Base(f.GeneratedFilenamePrefix)
		if err = generateFileAndExecuteTemplate(plugin, "", []string{
			"fmt",
			"google.golang.org/grpc",
		}, path.Join(cmdsDirectory, baseName+"_registry.go"), RegistryTemplate, f); err != nil {
			return fmt.Errorf("create registry for %q: %w", baseName, err)
		}
	}

	if err = generateFileAndExecuteTemplate(plugin, "", nil, path.Join(cmdsDirectory, "Dockerfile"), CmdDockerfileTemplate, plugin); err != nil {
		return fmt.Errorf("generate cmd Dockerfile: %w", err)
	}

	if err = generateFileAndExecuteTemplate(plugin, "", nil, path.Join(cmdsDirectory, "Makefile"), CmdMakefileTemplate, plugin); err != nil {
		return fmt.Errorf("generate cmd Makefile: %w", err)
	}

	return nil
}

func generateFile(plugin *protogen.Plugin, f *protogen.File) error {
	filename := f.GeneratedFilenamePrefix + "_grpcmock.pb.go"
	return generateFileAndExecuteTemplate(plugin, f.GoImportPath, []string{
		"context",
		"github.com/torqio/grpcmock/pkg/stub",
	}, filename, MockServerTemplate, f)
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
			if len(f.Services) == 0 {
				continue
			}

			if err := generateFile(plugin, f); err != nil {
				return fmt.Errorf("generate file %q: %w", f.GeneratedFilenamePrefix, err)
			}
			generated = true
		}

		if !generated {
			log("nothing generated, not generating grpcmock cmds.")
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
