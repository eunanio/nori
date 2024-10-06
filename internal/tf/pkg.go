package tf

import "os/exec"

// Check if terrafrom is installed
func checkTerrafromInstalled() bool {
	cmd := exec.Command("terraform", "--version")
	err := cmd.Run()
	return err == nil
}

// Check if tofu is installed
func checkTofuInstalled() bool {
	cmd := exec.Command("tofu", "--version")
	err := cmd.Run()
	return err == nil
}

func GetInstalledRuntime() string {
	if checkTerrafromInstalled() {
		return "terraform"
	}
	if checkTofuInstalled() {
		return "tofu"
	}
	return ""
}
