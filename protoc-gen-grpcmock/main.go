package main

import (
	"crypto/md5"
	_ "embed"
	"encoding/hex"
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

//go:embed server_tmpl/mock_method_unary.tmpl
var MockServerUnaryMethodTemplate string

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

func generateFileAndExecuteTemplate(plugin *protogen.Plugin, goImportPath protogen.GoImportPath, manualImports []string, filename string, templates []string, templateData any) error {
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
	isStreamingClient := func(method *protogen.Method) bool {
		return method.Desc.IsStreamingClient()
	}
	isStreamingServer := func(method *protogen.Method) bool {
		return method.Desc.IsStreamingServer()
	}
	isStreaming := func(method *protogen.Method) bool {
		return isStreamingClient(method) || isStreamingServer(method)
	}

	for _, manualImport := range manualImports {
		// This will make protogen to import this package (without '_' prefix)
		qualifiedIdentCustom(protogen.GoImportPath(manualImport), "")
	}

	funcs := template.FuncMap{
		"qualifiedIdent":       qualifiedIdent,
		"qualifiedIdentCustom": qualifiedIdentCustom,
		"isStreamingClient":    isStreamingClient,
		"isStreamingServer":    isStreamingServer,
		"isStreaming":          isStreaming,
	}

	var finalTemplate *template.Template
	var err error
	baseTemplate := template.New(filename).Funcs(funcs).Funcs(sprig.TxtFuncMap()).Option("missingkey=error")
	for _, tmplText := range templates {
		if finalTemplate != nil {
			finalTemplate, err = finalTemplate.Parse(tmplText)
		} else {
			finalTemplate, err = baseTemplate.Parse(tmplText)
		}
		if err != nil {
			return fmt.Errorf("parse template: %w", err)
		}
	}
	if err := finalTemplate.Execute(generatedFile, templateData); err != nil {
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

// shortGeneratedFileIdentifier generates a short hash identifier for the file's GeneratedFilenamePrefix.
// It required because the cmds can be at a root directory and multiple generated protos may have the same file name,
// and if it won't have some unique ID it will be overridden by the latest generated proto with the same name.
func shortGeneratedFileIdentifier(f *protogen.File) string {
	hexSum := md5.Sum([]byte(f.GeneratedFilenamePrefix))
	return hex.EncodeToString(hexSum[:])[:5]
}

func generateCmds(plugin *protogen.Plugin, cmdsPath string) error {
	var err error
	basePath := cmdsPath
	if basePath == "" {
		basePath, err = findBasePath(plugin)
		if err != nil {
			return err
		}
	}
	cmdsDirectory := path.Join(basePath, "grpcmock_cmds")

	if err = generateFileAndExecuteTemplate(plugin, "", nil, path.Join(cmdsDirectory, "server.mockpb.go"), []string{CmdServerTemplate}, plugin); err != nil {
		return fmt.Errorf("generate cmd server: %w", err)
	}

	for _, f := range plugin.Files {
		if !f.Generate {
			continue
		}
		if len(f.Services) == 0 {
			continue
		}

		baseName := fmt.Sprintf("%s_%s_registry.mockpb.go", shortGeneratedFileIdentifier(f), path.Base(f.GeneratedFilenamePrefix))
		if err = generateFileAndExecuteTemplate(plugin, "", []string{
			"fmt",
			"google.golang.org/grpc",
		}, path.Join(cmdsDirectory, baseName), []string{RegistryTemplate}, f); err != nil {
			return fmt.Errorf("create registry for %q: %w", baseName, err)
		}
	}

	if err = generateFileAndExecuteTemplate(plugin, "", nil, path.Join(cmdsDirectory, "Dockerfile"), []string{CmdDockerfileTemplate}, plugin); err != nil {
		return fmt.Errorf("generate cmd Dockerfile: %w", err)
	}

	if err = generateFileAndExecuteTemplate(plugin, "", nil, path.Join(cmdsDirectory, "Makefile"), []string{CmdMakefileTemplate}, plugin); err != nil {
		return fmt.Errorf("generate cmd Makefile: %w", err)
	}

	return nil
}

func generateFile(plugin *protogen.Plugin, f *protogen.File) error {
	//for _, service := range f.Services {
	//	os.Stderr.WriteString(fmt.Sprintf("Generating mock server for service %q\n", service.Desc.FullName()))
	//	for _, method := range service.Methods {
	//		os.Stderr.WriteString(fmt.Sprintf("  %s\n", method.Desc.FullName()))
	//		os.Stderr.WriteString(fmt.Sprintf("  input: %#v\n", method.Input))
	//		os.Stderr.WriteString(fmt.Sprintf("  output: %#v\n", method.Input))
	//		os.Stderr.WriteString(fmt.Sprintf("  method: %#v\n", method))
	//		os.Stderr.WriteString("========================================\n")
	//	}
	//}
	filename := f.GeneratedFilenamePrefix + "_grpcmock.pb.go"
	return generateFileAndExecuteTemplate(plugin, f.GoImportPath, []string{
		"context",
		"fmt",
		"testing",
		"errors",
		"io",
		"github.com/torqio/grpcmock/pkg/mocker",
		"google.golang.org/grpc",
		"google.golang.org/grpc/codes",
		"google.golang.org/grpc/status",
	}, filename, []string{MockServerTemplate, MockServerUnaryMethodTemplate}, f)
}

func main() {
	var flags flag.FlagSet
	shouldGenerateCmds := flags.Bool("generate-cmds", false, "Generate cmds main packages for mocked services")
	cmdsPath := flags.String("cmds-path", "", "Path to generate to cmds for the mocked services")
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
			return nil
		}

		if *shouldGenerateCmds {
			if err := generateCmds(plugin, *cmdsPath); err != nil {
				return fmt.Errorf("getnerate cmds: %w", err)
			}
		}
		return nil
	})
}
