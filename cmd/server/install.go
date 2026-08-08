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
		fmt.Println("Installing Chromium and GStreamer dependencies...")
		// update apt first
		if err := exec.Command("apt-get", "update").Run(); err != nil {
			log.Printf("Warning: Failed to run apt-get update: %v", err)
		}

		cmd := exec.Command("apt-get", "install", "-y", "chromium-common", "chromium", "chromium-headless-shell", "chromium-driver", "chromium-lwn4chrome", "chromium-sandbox", "chromium-shell", "gstreamer1.0-tools", "gstreamer1.0-plugins-base", "gstreamer1.0-plugins-good", "gstreamer1.0-plugins-bad", "gstreamer1.0-plugins-ugly", "gstreamer1.0-libav")
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
	fmt.Println("1) Create dedicated user (visionbridge) [default]")
	for i, u := range users {
		fmt.Printf("%d) Use existing user '%s'\n", i+2, u)
	}
	fmt.Print("Choice: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(scanner.Text())

	selectedUser := "visionbridge"
	if choice != "" && choice != "1" && choice != "visionbridge" {
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
				fmt.Println("Invalid choice, defaulting to visionbridge.")
			}
		} else {
			fmt.Println("Invalid choice, defaulting to visionbridge.")
		}
	}

	fmt.Println("Selected user:", selectedUser)
	return selectedUser
}

func setupUserAndSettings(installBase, etcDir, varDir, selectedUser string) {
	if selectedUser == "visionbridge" {
		cmd := exec.Command("id", "-u", "visionbridge")
		if err := cmd.Run(); err != nil {
			fmt.Println("Creating user visionbridge...")
			if err := exec.Command("useradd", "-m", "-s", "/bin/bash", "visionbridge").Run(); err != nil {
				log.Fatalf("Failed to create user visionbridge: %v", err)
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
			// Update database DSN and connector group
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
				}
				if mapping.Content[i].Value == "connector" {
					connectorMapping := mapping.Content[i+1]
					if connectorMapping.Kind == yaml.MappingNode {
						for j := 0; j < len(connectorMapping.Content); j += 2 {
							if connectorMapping.Content[j].Value == "group" {
								connectorMapping.Content[j+1].Value = selectedUser
								break
							}
						}
					}
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

	fmt.Println("Removing pipewire and disabling lingering for users...")
	if err := exec.Command("apt-get", "purge", "-y", "pipewire", "wireplumber", "pipewire-pulse", "pipewire-audio").Run(); err != nil {
		log.Printf("Warning: Failed to purge pipewire packages: %v", err)
	}
	if err := exec.Command("apt-get", "autoremove", "-y").Run(); err != nil {
		log.Printf("Warning: Failed to autoremove packages: %v", err)
	}

	for _, u := range users {
		_ = exec.Command("loginctl", "disable-linger", u).Run()
	}
	if selectedUser != "visionbridge" {
		_ = exec.Command("loginctl", "disable-linger", "visionbridge").Run()
	}
	_ = exec.Command("loginctl", "disable-linger", selectedUser).Run()

	promptChromiumInstall()

	generateSystemdService(selectedUser, installBase)

	fmt.Println("Installation complete.")
}

func generateSystemdService(selectedUser, installBase string) {
	fmt.Println("Generating systemd service for user:", selectedUser)

	serviceContent := fmt.Sprintf(`[Unit]
Description=VLX VisionBridge Service
After=network.target sound.target

[Service]
Type=simple
User=%s
Environment="DISPLAY=:99"
ExecStart=%s/bin/VLX_VisionBridge
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
`, selectedUser, installBase)

	err := os.WriteFile("/etc/systemd/system/visionbridge.service", []byte(serviceContent), 0644)
	if err != nil {
		log.Printf("Failed to write systemd service file: %v", err)
		return
	}

	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		log.Printf("Failed to reload systemd daemon: %v", err)
		return
	}

	if err := exec.Command("systemctl", "enable", "visionbridge.service").Run(); err != nil {
		log.Printf("Failed to enable visionbridge.service: %v", err)
		return
	}

	fmt.Println("Successfully generated and enabled visionbridge.service")
}
