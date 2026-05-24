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

	"github.com/user/VLX_VisionBridge/configs"
	"github.com/user/VLX_VisionBridge/internal/db"
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

func setupDirectories(binDir, etcDir, varDir string) {
	if err := os.MkdirAll(binDir, 0755); err != nil {
		log.Fatalf("Failed to create bin dir: %v", err)
	}
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		log.Fatalf("Failed to create etc dir: %v", err)
	}
	if err := os.MkdirAll(varDir, 0755); err != nil {
		log.Fatalf("Failed to create var dir: %v", err)
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
	if err := copyTemplate(configs.SettingsTemplate, configPath); err != nil {
		log.Fatalf("Failed to handle visionbridge.settings: %v", err)
	}
	fmt.Println("Configured settings template at", configPath)
}

func promptChromiumInstall() {
	fmt.Print("\nDo you want to install Chromium for overlay support? (y/N): ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(strings.ToLower(scanner.Text()))

	if choice == "y" || choice == "yes" {
		fmt.Println("Installing Chromium and Xvfb dependencies...")
		// update apt first
		if err := exec.Command("apt-get", "update").Run(); err != nil {
			log.Printf("Warning: Failed to run apt-get update: %v", err)
		}

		cmd := exec.Command("apt-get", "install", "-y", "xvfb", "chromium-common", "chromium", "chromium-headless-shell", "chromium-driver", "chromium-lwn4chrome", "chromium-sandbox", "chromium-shell")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Printf("Warning: Failed to install Chromium dependencies: %v", err)
		} else {
			fmt.Println("Chromium installation complete.")
		}
	} else {
		fmt.Println("Skipping Chromium installation.")
	}
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
	if choice != "" && choice != "1" && choice != "VisionBridge" {
		idx, err := strconv.Atoi(choice)
		if err == nil && idx >= 2 && idx <= len(users)+1 {
			selectedUser = users[idx-2]
		} else if err != nil {
			found := false
			for _, u := range users {
				if choice == u {
					selectedUser = u
					found = true
					break
				}
			}
			if !found {
				fmt.Println("Invalid choice, defaulting to VisionBridge.")
			}
		} else {
			fmt.Println("Invalid choice, defaulting to VisionBridge.")
		}
	}

	fmt.Println("Selected user:", selectedUser)
	return selectedUser
}

func setupUserAndSettings(installBase, etcDir, varDir, selectedUser string) {
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
			foundUser := false
			foundDir := false
			for i := 0; i < len(mapping.Content); i += 2 {
				if mapping.Content[i].Value == "visionbridge_USER" {
					mapping.Content[i+1].Value = selectedUser
					foundUser = true
				}
				if mapping.Content[i].Value == "visionbridge_DIR" {
					mapping.Content[i+1].Value = "/opt/VLX_VisionBridge"
					foundDir = true
				}
			}
			if !foundUser {
				keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "visionbridge_USER"}
				valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: selectedUser}
				mapping.Content = append(mapping.Content, keyNode, valNode)
			}
			if !foundDir {
				keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "visionbridge_DIR"}
				valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "/opt/VLX_VisionBridge"}
				mapping.Content = append(mapping.Content, keyNode, valNode)
			}

			// Update database DSN
			for i := 0; i < len(mapping.Content); i += 2 {
				if mapping.Content[i].Value == "database" {
					dbMapping := mapping.Content[i+1]
					if dbMapping.Kind == yaml.MappingNode {
						for j := 0; j < len(dbMapping.Content); j += 2 {
							if dbMapping.Content[j].Value == "dsn" {
								dbMapping.Content[j+1].Value = "/opt/VLX_VisionBridge/var/visionbridge.db"
								break
							}
						}
					}
					break
				}
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

	fmt.Println("Changing ownership of", installBase, "to", selectedUser)
	if err := exec.Command("chown", "-R", selectedUser+":"+selectedUser, installBase).Run(); err != nil {
		log.Fatalf("Failed to chown %s: %v", installBase, err)
	}
}

func Install() {
	fmt.Println("Starting installation of VLX_VisionBridge...")

	installBase := "/opt/VLX_VisionBridge"
	binDir := filepath.Join(installBase, "bin")
	etcDir := filepath.Join(installBase, "etc")
	varDir := filepath.Join(installBase, "var")

	setupDirectories(binDir, etcDir, varDir)

	copyExecutable(binDir)

	setupConfig(etcDir)

	dbPath := filepath.Join(varDir, "visionbridge.db")
	dbConn, err := db.InitDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database at %s: %v", dbPath, err)
	}
	if err := db.SetupTables(dbConn); err != nil {
		log.Fatalf("Failed to setup database tables: %v", err)
	}
	dbConn.Close()

	users := getEligibleUsers()
	selectedUser := promptUser(users)

	setupUserAndSettings(installBase, etcDir, varDir, selectedUser)

	promptChromiumInstall()

	fmt.Println("Installation complete.")
}
