package stub

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/nsf/jsondiff"
	"github.com/oriser/regroup"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type MethodFileStub struct {
	RequestFilePath  string
	ResponseFilePath string
}
type MethodFileStubs map[string][]MethodFileStub

type fileMethodRegexpGroup struct {
	Method string `regroup:"method"`
}

var fileNameTemplateRegexp = regroup.MustCompile(`(?:.*?__)?(?P<method>.+?)__request\.json`)

const requestSuffix = "_request.json"
const responseSuffix = "_response.json"

func MapStubFiles(rootStubsDir string) (MethodFileStubs, error) {
	dirStat, err := os.Stat(rootStubsDir)
	if err != nil {
		return nil, fmt.Errorf("stat stubs directory %q: %w", rootStubsDir, err)
	}
	if !dirStat.IsDir() {
		return nil, fmt.Errorf("path %q for stubs must be a directory", rootStubsDir)
	}

	stubFiles := make(MethodFileStubs)

	return stubFiles, filepath.Walk(rootStubsDir, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		dirName := filepath.Dir(path)
		fileName := filepath.Base(path)

		if strings.HasSuffix(fileName, responseSuffix) {
			// Skipping response files without logging
			return nil
		}
		if !strings.HasSuffix(fileName, requestSuffix) {
			log.Printf("Skipping file %q as it doesn't have %q suffix\n", path, requestSuffix)
			return nil
		}

		responseFile := fileName[:len(fileName)-len(requestSuffix)] + responseSuffix // replacing _request.json with _response.json
		responseFullPath := filepath.Join(dirName, responseFile)

		if _, err = os.Stat(responseFullPath); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("found request file %q, but expected response file %q wasn't found", path, responseFullPath)
			}
			return fmt.Errorf("stat response file: %w", err)
		}

		fileMethod := fileMethodRegexpGroup{}
		reqFilename := filepath.Base(path)
		if err = fileNameTemplateRegexp.MatchToTarget(reqFilename, &fileMethod); err != nil {
			if errors.Is(err, &regroup.NoMatchFoundError{}) {
				return fmt.Errorf("request file %q doesn't contains method name. Request file must be in the"+
					" following format: [description__]<RPC method name>__request.json. For example: \"some description__CreateAccount__request.json\"",
					reqFilename)
			}
			return fmt.Errorf("match re to path %q: %w", reqFilename, err)
		}

		stubFiles[fileMethod.Method] = append(stubFiles[fileMethod.Method], MethodFileStub{RequestFilePath: path, ResponseFilePath: responseFullPath})
		return nil
	})
}

func GetFileStubResponse(stubs MethodFileStubs, method string, req proto.Message, res proto.Message) error {
	stubFiles := stubs[method]
	if len(stubFiles) == 0 {
		return fmt.Errorf("not stubs for %q", method)
	}

	gotJSON, err := protojson.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request JSON: %w", err)
	}

	for _, stubFile := range stubFiles {
		stubReqJSON, err := os.ReadFile(stubFile.RequestFilePath)
		if err != nil {
			return fmt.Errorf("read stub request %q: %w", stubFile.RequestFilePath, err)
		}

		if !json.Valid(stubReqJSON) {
			return fmt.Errorf("stub file %q contains an invalid JSON", stubFile.RequestFilePath)
		}

		o := jsondiff.DefaultJSONOptions()
		compareRes, _ := jsondiff.Compare(gotJSON, stubReqJSON, &o)
		if compareRes != jsondiff.FullMatch && compareRes != jsondiff.SupersetMatch {
			// We consider a request as a match in case it's a full match (JSON is identical) or it's a superset -
			// means the stub is a subset of the request JSON
			// (because we may have fields which we don't want to compare like date fields)
			continue
		}

		stubResponseJSON, err := os.ReadFile(stubFile.ResponseFilePath)
		if err != nil {
			return fmt.Errorf("read stub response %q: %w", stubFile.ResponseFilePath, err)
		}

		if err := protojson.Unmarshal(stubResponseJSON, res); err != nil {
			return fmt.Errorf("unmarshal stub response into provided response type: %w", err)
		}

		return nil
	}

	return fmt.Errorf("no matching stub found for the provided request")
}
