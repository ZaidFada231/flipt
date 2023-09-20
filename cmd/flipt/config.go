package main

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"go.flipt.io/flipt/internal/config"
	"gopkg.in/yaml.v2"
)

type configCmd struct {
}

func newConfigCommand() *cobra.Command {
	configCmd := &configCmd{}

	var cmd = &cobra.Command{
		Use:   "config",
		Short: "Manage Flipt configuration",
	}

	var initCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize Flipt configuration",
		RunE:  configCmd.init,
	}

	var editCmd = &cobra.Command{
		Use:   "edit",
		Short: "Edit Flipt configuration",
		RunE:  configCmd.edit,
	}

	cmd.AddCommand(initCmd)
	cmd.AddCommand(editCmd)
	return cmd
}

func (c *configCmd) init(cmd *cobra.Command, args []string) error {
	cfg := config.Default()
	cfg.Version = config.Version // write version for backward compatibility
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	var file string

	q := []*survey.Question{
		{
			Name: "file",
			Prompt: &survey.Input{
				Message: "Configuration file path:",
				Default: fliptConfigFile,
			},
			Validate: survey.Required,
		},
	}

	if err := survey.Ask(q, &file); err != nil {
		return err
	}

	// check if file exists
	if _, err := os.Stat(file); err == nil {
		// file exists
		overwrite := false
		prompt := &survey.Confirm{
			Message: "File exists. Overwrite?",
		}
		if err := survey.AskOne(prompt, &overwrite); err != nil {
			return err
		}
	}

	return os.WriteFile(file, out, 0600)
}

func (c *configCmd) edit(cmd *cobra.Command, args []string) error {
	b, err := os.ReadFile(fliptConfigFile)
	if err != nil {
		return fmt.Errorf("reading config file %w", err)
	}

	content := string(b)

	if err := survey.AskOne(&survey.Editor{
		Message:       "Edit configuration",
		Default:       string(b),
		HideDefault:   true,
		AppendDefault: true,
	}, &content, nil); err != nil {
		return err
	}

	// open file for writing
	f, err := os.OpenFile(fliptConfigFile, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("opening config file %w", err)
	}

	defer f.Close()

	_, err = f.WriteString(content)
	if err != nil {
		return fmt.Errorf("writing config file %w", err)
	}
	return nil
}
