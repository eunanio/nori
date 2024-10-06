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

func Plan(path string) error {
	config := config.Load()
	if config == nil {
		fmt.Println("error: config not found, ensure you have run nori init?")
		return fmt.Errorf("error: config not found, ensure you have run nori init?")
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
	err := exe.ExecuteWithStream(init_opts)
	if err != nil {
		return fmt.Errorf("error init: %s", err)
	}

	err = exe.ExecuteWithStream(plan_opts)
	if err != nil {
		return fmt.Errorf("error plan: %s", err)
	}

	return nil
}

func Apply(path string) error {
	config := config.Load()
	if config == nil {
		fmt.Println("error: config not found, ensure you have run nori init?")
		return nil
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
	err := exe.ExecuteWithStream(init_opts)
	if err != nil {
		return err
	}

	err = exe.ExecuteWithStream(apply_opts)
	if err != nil {
		return err
	}
	return nil
}

func Destroy(path string) error {
	config := config.Load()
	if config == nil {
		fmt.Println("error: config not found, ensure you have run nori init?")
		return nil
	}

	exe := cmd.Cmd{}

	init_args := []string{"init"}
	init_opts := cmd.CmdArgs{
		Dir:  path,
		Run:  config.Runtime,
		Args: init_args,
	}

	destory_opts := cmd.CmdArgs{
		Dir:  path,
		Run:  config.Runtime,
		Args: []string{"apply", "-auto-approve", "-input=false", "-destroy"},
	}

	err := exe.ExecuteWithStream(init_opts)
	if err != nil {
		return err
	}

	err = exe.ExecuteWithStream(destory_opts)
	if err != nil {
		return err
	}
	return nil
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
