package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	internalapi "github.com/lxdb/busylib-go/internal/api"
)

var apiVersionPattern = regexp.MustCompile(`#define\s+API_VERSION\s+\{\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\}`)

func main() {
	firmwareDir := flag.String("firmware-dir", "", "path to the busybar-firmware checkout")
	contractPath := flag.String("contract", "internal/api/testdata/firmware-contract.json", "path to the firmware contract receipt")
	flag.Parse()

	if *firmwareDir == "" {
		fatalf("-firmware-dir is required")
	}
	contract, err := internalapi.LoadContractFile(*contractPath)
	if err != nil {
		fatalf("load contract: %v", err)
	}
	if err := checkFirmware(*firmwareDir, contract); err != nil {
		fatalf("firmware contract drift: %v", err)
	}
	fmt.Printf("firmware contract matches %s at %s (API %s, %d operations)\n", contract.Repository, contract.FirmwareCommit, contract.APIVersion, len(contract.Operations))
}

func checkFirmware(root string, contract internalapi.Contract) error {
	head, err := gitOutput(root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != contract.FirmwareCommit {
		return fmt.Errorf("HEAD = %s, audited commit = %s; audit the firmware diff before refreshing the receipt", head, contract.FirmwareCommit)
	}

	versionPath := filepath.Join(root, "applications/services/web_server/http_api/http_api.h")
	versionData, err := os.ReadFile(versionPath)
	if err != nil {
		return err
	}
	match := apiVersionPattern.FindStringSubmatch(string(versionData))
	if match == nil {
		return fmt.Errorf("API_VERSION is missing from %s", versionPath)
	}
	version := strings.Join(match[1:], ".")
	if version != contract.APIVersion {
		return fmt.Errorf("API_VERSION = %s, receipt = %s", version, contract.APIVersion)
	}

	protoTree, err := gitOutput(root, "ls-tree", "HEAD", "assets/proto")
	if err != nil {
		return err
	}
	fields := strings.Fields(protoTree)
	if len(fields) < 3 || fields[1] != "commit" {
		return fmt.Errorf("assets/proto is not a firmware gitlink: %q", protoTree)
	}
	if fields[2] != contract.ProtobufCommit {
		return fmt.Errorf("protobuf gitlink = %s, receipt = %s", fields[2], contract.ProtobufCommit)
	}

	checked := make(map[string][]byte)
	for _, operation := range contract.Operations {
		data, ok := checked[operation.SourceFile]
		if !ok {
			data, err = os.ReadFile(filepath.Join(root, filepath.FromSlash(operation.SourceFile)))
			if err != nil {
				return fmt.Errorf("%s: %w", operation.ID(), err)
			}
			checked[operation.SourceFile] = data
		}
		if !strings.Contains(string(data), operation.SourceSymbol) {
			return fmt.Errorf("%s source symbol %q is missing from %s", operation.ID(), operation.SourceSymbol, operation.SourceFile)
		}
	}
	return nil
}

func gitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output)), nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
