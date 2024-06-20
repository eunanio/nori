package tf

import (
	"fmt"

	"github.com/eunanhardy/nori/internal/config"
	"github.com/eunanhardy/nori/internal/tf/cmd"
)

type ApplyOpts struct {
	Path string
	Plan string
	Runtime string
}

func Plan(path string) (string, error) {
	config := config.Load()
	if config == nil {
		fmt.Println("error: config not found, ensure you have run nori init?")
		return "", nil
	}
	exe := cmd.Cmd{}
	init_opts := cmd.CmdArgs{
		Dir:  path,
		Run:  config.Runtime,
		Args: []string{"init"},
	}
	plan_opts := cmd.CmdArgs{
		Dir:  path,
		Run:  config.Runtime,
		Args: []string{"plan"},
	}
	_,errOut, err := exe.ExecuteWithErr(init_opts)
	if err != nil {
		return errOut, err
	}

	out, errOut, err := exe.ExecuteWithErr(plan_opts)
	if err != nil {
		return errOut, err
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
	init_opts := cmd.CmdArgs{
		Dir:  path,
		Run:  config.Runtime,
		Args: []string{"init"},
	}
	plan_opts := cmd.CmdArgs{
		Dir:  path,
		Run:  config.Runtime,
		Args: []string{"apply", "-auto-approve", "-input=false"},
	}
	_, err := exe.Execute(init_opts)
	if err != nil {
		return "", err
	}

	out, err := exe.Execute(plan_opts)
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