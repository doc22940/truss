package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/Sirupsen/logrus"

	"github.com/TuneLab/gob/truss/data"
)

const GENERATED_PATH = "service"
const GOOGLE_API_HTTP_IMPORT_PATH = "/service/DONOTEDIT/third_party/googleapis"

type globalStruct struct {
	workingDirectory string
	genImportPath    string
	GOPATH           string
	generatePbGoCmd  string
	generateDocsCmd  string
	generateGoKitCmd string
}

var global globalStruct

// We build up environment knowledge here
// 1. Get working directory
// 2. Get $GOPATH
// 3. Use 1,2 to build path for golang imports for this package
// 4. Build 3 proto commands to invoke
func init() {
	log.SetLevel(log.DebugLevel)

	var err error
	global.workingDirectory, err = os.Getwd()
	if err != nil {
		log.WithError(err).Fatal("Cannot get working directory")
	}

	global.GOPATH = os.Getenv("GOPATH")

	// From `$GOPATH/src/org/user/thing` get `org/user/thing` from importing in golang
	global.genImportPath = strings.TrimPrefix(global.workingDirectory, global.GOPATH+"/src/")

	// Generate grpc golang code
	global.generatePbGoCmd = "--go_out=Mgoogle/api/annotations.proto=" + global.genImportPath + GOOGLE_API_HTTP_IMPORT_PATH + "/google/api,plugins=grpc:./service/DONOTEDIT/pb"
	// Generate documentation
	global.generateDocsCmd = "--gendoc_out=."
	// Generate gokit-base service
	global.generateGoKitCmd = "--truss-gokit_out=."

}

// Stages are documented in README.md
func main() {
	flag.Parse()

	if len(flag.Args()) == 0 {
		fmt.Fprintf(os.Stderr, "usage: truss microservice.proto\n")
		os.Exit(1)
	}

	definitionPaths := flag.Args()

	// Stage 1
	global.buildDirectories()
	global.outputGoogleImport()

	// Stage 2, 3, 4
	global.protoc(definitionPaths, global.generatePbGoCmd)
	global.protoc(definitionPaths, global.generateDocsCmd)
	global.protoc(definitionPaths, global.generateGoKitCmd)

	// Stage 5
	goBuild("server", "./service/DONOTEDIT/cmd/svc/...")
	goBuild("cliclient", "./service/DONOTEDIT/cmd/cliclient/...")

}

// buildDirectories puts the following directories in place
// .
// └── service
//     ├── bin
//     └── DONOTEDIT
//         ├── pb
//         └── third_party
//             └── googleapis
//                 └── google
//                     └── api
func (g globalStruct) buildDirectories() {
	// third_party created by going through assets in data
	// and creating directoires that are not there
	for _, filePath := range data.AssetNames() {
		fullPath := g.workingDirectory + "/" + filePath

		dirPath := filepath.Dir(fullPath)

		err := os.MkdirAll(dirPath, 0777)
		if err != nil {
			log.WithField("DirPath", dirPath).WithError(err).Fatal("Cannot create directories")
		}
	}

	// Create the directory where protoc will store the compiled .pb.go files
	err := os.MkdirAll("service/DONOTEDIT/pb", 0777)
	if err != nil {
		log.WithField("DirPath", "service/DONOTEDIT/pb").WithError(err).Fatal("Cannot create directories")
	}

	// Create the directory where go build will put the compiled binaries
	err = os.MkdirAll("service/bin", 0777)
	if err != nil {
		log.WithField("DirPath", "service/bin").WithError(err).Fatal("Cannot create directories")
	}
}

// outputGoogleImport places imported and required google.api.http protobuf option files
// into their required directories as part of stage one generation
func (g globalStruct) outputGoogleImport() {
	// Output files that are stored in data package
	for _, filePath := range data.AssetNames() {
		fileBytes, _ := data.Asset(filePath)
		fullPath := g.workingDirectory + "/" + filePath

		err := ioutil.WriteFile(fullPath, fileBytes, 0666)
		if err != nil {
			log.WithField("FilePath", fullPath).WithError(err).Fatal("Cannot create ")
		}
	}
}

// goBuild calls the `$ go get ` to install dependenices
// and then calls `$ go build service/bin/$name $path`
// to put the iterating binaries in the correct place
func goBuild(name string, path string) {

	goGetExec := exec.Command(
		"go",
		"get",
		"-d",
		"-v",
		path,
	)

	goGetExec.Stderr = os.Stderr

	log.WithField("cmd", strings.Join(goGetExec.Args, " ")).Info("go get")
	val, err := goGetExec.Output()

	if err != nil {
		log.WithFields(log.Fields{
			"output": string(val),
			"input":  goGetExec.Args,
		}).WithError(err).Fatal("go get failed")
	}

	goBuildExec := exec.Command(
		"go",
		"build",
		"-o",
		"service/bin/"+name,
		path,
	)
	//env := os.Environ()
	//env = append(env, "CGO_ENABLED=0")
	//goBuildExec.Env = env

	goBuildExec.Stderr = os.Stderr

	log.WithField("cmd", strings.Join(goBuildExec.Args, " ")).Info("go build")
	val, err = goBuildExec.Output()

	if err != nil {
		log.WithFields(log.Fields{
			"output": string(val),
			"input":  goBuildExec.Args,
		}).WithError(err).Fatal("go build failed")
	}
}

func (g globalStruct) protoc(definitionPaths []string, command string) {
	cmdArgs := []string{
		"-I.",
		"-I" + g.workingDirectory + GOOGLE_API_HTTP_IMPORT_PATH,
		command,
	}
	// Append each definition file path to the end of that command args
	cmdArgs = append(cmdArgs, definitionPaths...)

	protocExec := exec.Command(
		"protoc",
		cmdArgs...,
	)

	protocExec.Stderr = os.Stderr

	log.WithField("cmd", strings.Join(protocExec.Args, " ")).Info("protoc")
	val, err := protocExec.Output()

	if err != nil {
		log.WithFields(log.Fields{
			"output": string(val),
			"input":  protocExec.Args,
		}).WithError(err).Fatal("Protoc call failed")
	}
}
