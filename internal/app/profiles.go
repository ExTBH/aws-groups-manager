package app

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func loadProfiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	paths := []string{
		filepath.Join(home, ".aws", "config"),
		filepath.Join(home, ".aws", "credentials"),
	}

	profiles := make(map[string]struct{})
	for _, p := range paths {
		if err := readProfilesFromFile(p, profiles); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
	}

	list := make([]string, 0, len(profiles))
	for profile := range profiles {
		list = append(list, profile)
	}
	sort.Strings(list)
	return list, nil
}

func readProfilesFromFile(path string, out map[string]struct{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
			continue
		}

		name := strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
		name = strings.TrimSpace(name)
		name = strings.TrimPrefix(name, "profile ")
		if name != "" {
			out[name] = struct{}{}
		}
	}

	return scanner.Err()
}

var awsRegions = []string{
	"af-south-1",
	"ap-east-1",
	"ap-south-1",
	"ap-south-2",
	"ap-southeast-1",
	"ap-southeast-2",
	"ap-southeast-3",
	"ap-southeast-4",
	"ap-northeast-1",
	"ap-northeast-2",
	"ap-northeast-3",
	"ca-central-1",
	"eu-central-1",
	"eu-central-2",
	"eu-west-1",
	"eu-west-2",
	"eu-west-3",
	"eu-north-1",
	"eu-south-1",
	"eu-south-2",
	"il-central-1",
	"me-central-1",
	"me-south-1",
	"sa-east-1",
	"us-east-1",
	"us-east-2",
	"us-west-1",
	"us-west-2",
}
