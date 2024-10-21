package install_name_tool

import (
	"os/exec"
)

// InstallNameTool is a struct for install_name_tool command

func Change(old string, new string, file string) error {
	cmd := exec.Command("install_name_tool", "-change", old, new, file)
	return cmd.Run()
}

func ChangeId(new string, file string) error {
	cmd := exec.Command("install_name_tool", "-id", new, file)
	return cmd.Run()
}
