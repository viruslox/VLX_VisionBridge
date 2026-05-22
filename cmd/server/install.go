package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/user/VLX_VisionBridge/internal/assets"
	"gopkg.in/yaml.v3"
)

func parseEligibleUsers(data []byte) []string {
	var users []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) >= 7 {
			uid, _ := strconv.Atoi(parts[2])
			shell := parts[6]
			if uid >= 1000 && (shell == "/bin/bash" || shell == "/bin/zsh") {
				users = append(users, parts[0])
			}
		}
	}
	return users
}

func getEligibleUsers() []string {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return nil
	}
	return parseEligibleUsers(data)
}

func addMissingKeys(dest, tmpl *yaml.Node) {
	if tmpl.Kind == yaml.MappingNode && dest.Kind == yaml.MappingNode {
		destMap := make(map[string]int, len(dest.Content)/2)
		for j := 0; j < len(dest.Content); j += 2 {
			destMap[dest.Content[j].Value] = j
		}

		for i := 0; i < len(tmpl.Content); i += 2 {
			tmplKey := tmpl.Content[i]
			tmplVal := tmpl.Content[i+1]

			if j, found := destMap[tmplKey.Value]; found {

				if tmplVal.Kind == yaml.MappingNode && dest.Content[j+1].Kind == yaml.MappingNode {
					addMissingKeys(dest.Content[j+1], tmplVal)

				}
			} else {
				dest.Content = append(dest.Content, tmplKey, tmplVal)
			}
		}
	}
}

func copyTemplate(templateContent []byte, destPath string) error {
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return os.WriteFile(destPath, templateContent, 0600)
	}

	existingContent, err := os.ReadFile(destPath)
	if err != nil {
		return err
	}

	var tmplNode yaml.Node
	if err := yaml.Unmarshal(templateContent, &tmplNode); err != nil {
		return err
	}
	var destNode yaml.Node
	if err := yaml.Unmarshal(existingContent, &destNode); err != nil {
		return err
	}

	if len(tmplNode.Content) > 0 && len(destNode.Content) > 0 {
		addMissingKeys(destNode.Content[0], tmplNode.Content[0])
	}

	out, err := yaml.Marshal(&destNode)
	if err != nil {
		return err
	}

	return os.WriteFile(destPath, out, 0600)
}

func setupDirectories(binDir, etcDir string) {
	if err := os.MkdirAll(binDir, 0755); err != nil {
		log.Fatalf("Failed to create bin dir: %v", err)
	}
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		log.Fatalf("Failed to create etc dir: %v", err)
	}
}

func copyExecutable(binDir string) {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	destExe := filepath.Join(binDir, "VLX_VisionBridge")
	exeData, err := os.ReadFile(exePath)
	if err != nil {
		log.Fatalf("Failed to read executable: %v", err)
	}
	if err := os.WriteFile(destExe, exeData, 0755); err != nil {
		log.Fatalf("Failed to write executable: %v", err)
	}
	fmt.Println("Copied executable to", destExe)
}

func setupConfig(etcDir string) {
	configPath := filepath.Join(etcDir, "visionbridge.settings")
	if err := copyTemplate(assets.SettingsTemplate, configPath); err != nil {
		log.Fatalf("Failed to handle visionbridge.settings: %v", err)
	}
	fmt.Println("Configured settings template at", configPath)
}

func promptUser(users []string) string {
	fmt.Println("\nSelect user to run VLX_VisionBridge:")
	fmt.Println("1) Create dedicated user (VisionBridge) [default]")
	for i, u := range users {
		fmt.Printf("%d) Use existing user '%s'\n", i+2, u)
	}
	fmt.Print("Choice: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())

	selectedUser := "VisionBridge"
	if choice != "" && choice != "1" {
		idx, err := strconv.Atoi(choice)
		if err == nil && idx >= 2 && idx <= len(users)+1 {
			selectedUser = users[idx-2]
		} else {
			fmt.Println("Invalid choice, defaulting to VisionBridge.")
		}
	}

	fmt.Println("Selected user:", selectedUser)
	return selectedUser
}

func setupUserAndSettings(etcDir, selectedUser string) {
	if selectedUser == "VisionBridge" {
		cmd := exec.Command("id", "-u", "VisionBridge")
		if err := cmd.Run(); err != nil {
			fmt.Println("Creating user VisionBridge...")
			if err := exec.Command("useradd", "-m", "-s", "/bin/bash", "VisionBridge").Run(); err != nil {
				log.Fatalf("Failed to create user VisionBridge: %v", err)
			}
		}
	}

	settingsPath := filepath.Join(etcDir, "visionbridge.settings")
	existingContent, err := os.ReadFile(settingsPath)
	if err != nil {
		log.Fatalf("Failed to read visionbridge.settings: %v", err)
	}

	var destNode yaml.Node
	if err := yaml.Unmarshal(existingContent, &destNode); err != nil {
		log.Fatalf("Failed to unmarshal visionbridge.settings: %v", err)
	}

	if destNode.Kind == yaml.DocumentNode && len(destNode.Content) > 0 {
		mapping := destNode.Content[0]
		if mapping.Kind == yaml.MappingNode {
			found := false
			for i := 0; i < len(mapping.Content); i += 2 {
				if mapping.Content[i].Value == "visionbridge_USER" {
					mapping.Content[i+1].Value = selectedUser
					found = true
					break
				}
			}
			if !found {
				keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "visionbridge_USER"}
				valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: selectedUser}
				mapping.Content = append(mapping.Content, keyNode, valNode)
			}
		}
	}

	out, err := yaml.Marshal(&destNode)
	if err != nil {
		log.Fatalf("Failed to marshal visionbridge.settings: %v", err)
	}

	if err := os.WriteFile(settingsPath, out, 0600); err != nil {
		log.Fatalf("Failed to write visionbridge.settings: %v", err)
	}
	fmt.Println("Updated visionbridge.settings")

	fmt.Println("Changing ownership of", etcDir, "to", selectedUser)
	if err := exec.Command("chown", "-R", selectedUser+":"+selectedUser, etcDir).Run(); err != nil {
		log.Fatalf("Failed to chown: %v", err)
	}
}

func Install() {
	fmt.Println("Starting installation of VLX_VisionBridge...")

	installBase := "/opt/VLX_VisionBridge"
	binDir := filepath.Join(installBase, "bin")
	etcDir := filepath.Join(installBase, "etc")

	setupDirectories(binDir, etcDir)

	copyExecutable(binDir)

	setupConfig(etcDir)

	users := getEligibleUsers()
	selectedUser := promptUser(users)

	setupUserAndSettings(etcDir, selectedUser)

	fmt.Println("Installation complete.")
}
