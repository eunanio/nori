package tf

import (
	"fmt"

	"github.com/eunanio/nori/internal/config"
	"github.com/eunanio/nori/internal/tf/cmd"
)

type ApplyOpts struct {
	Path    string
	Plan    string
	Runtime string
}

func Plan(path string) (string, error) {
	config := config.Load()
	if config == nil {
		fmt.Println("error: config not found, ensure you have run nori init?")
		return "", fmt.Errorf("error: config not found, ensure you have run nori init?")
	}
	exe := cmd.Cmd{}

	init_args := []string{"init"}

	init_opts := cmd.CmdArgs{
		Dir:  path,
		Run:  config.Runtime,
		Args: init_args,
	}
	plan_opts := cmd.CmdArgs{
		Dir:  path,
		Run:  config.Runtime,
		Args: []string{"plan"},
	}
	out, err := exe.Execute(init_opts)
	if err != nil {
		return out, err
	}

	out, err = exe.Execute(plan_opts)
	if err != nil {
		return out, err
	}
	return out, nil
}

func Apply(path string) (string, error) {
	config := config.Load()
	if config == nil {
		fmt.Println("error: config not found, ensure you have run nori init?")
		return "", nil
	}

	exe := cmd.Cmd{}

	init_args := []string{"init"}
	init_opts := cmd.CmdArgs{
		Dir:  path,
		Run:  config.Runtime,
		Args: init_args,
	}
	apply_opts := cmd.CmdArgs{
		Dir:  path,
		Run:  config.Runtime,
		Args: []string{"apply", "-auto-approve", "-input=false"},
	}
	out, err := exe.Execute(init_opts)
	if err != nil {
		return out, err
	}

	out, err = exe.Execute(apply_opts)
	if err != nil {
		return out, err
	}
	return out, nil
}

func Destroy(path string) (string, error) {
	config := config.Load()
	if config == nil {
		fmt.Println("error: config not found, ensure you have run nori init?")
		return "", nil
	}

	exe := cmd.Cmd{}

	destory_opts := cmd.CmdArgs{
		Dir:  path,
		Run:  config.Runtime,
		Args: []string{"apply", "-auto-approve", "-input=false", "-destroy"},
	}

	out, err := exe.Execute(destory_opts)
	if err != nil {
		return out, err
	}
	return out, nil
}

func Output(path string) (string, error) {
	config := config.Load()
	if config == nil {
		fmt.Println("error: config not found, ensure you have run nori init?")
		return "", nil
	}
	exe := cmd.Cmd{}
	opts := cmd.CmdArgs{
		Dir:  path,
		Run:  config.Runtime,
		Args: []string{"output"},
	}
	out, err := exe.Execute(opts)
	if err != nil {
		return out, err
	}
	return out, nil
}
