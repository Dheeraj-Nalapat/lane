package cmd

import (
	"errors"
	"fmt"

	"github.com/dheerajnalapat/lane/internal/doctor"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check that the environment is ready for lane",
	RunE: func(cmd *cobra.Command, args []string) error {
		report, ok := doctor.Report()
		fmt.Print(report)
		if !ok {
			return errors.New("some checks failed")
		}
		return nil
	},
}

func init() { root.AddCommand(doctorCmd) }
