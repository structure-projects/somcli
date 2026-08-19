/*
Copyright 2023 Structure Projects

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/structure-projects/somcli/pkg/state"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show what somcli has installed, and where",
	Long: `List the install records somcli keeps in <workdir>/state.json.

Each row is one resource on one target: a resource without hosts: is recorded
against "(local)", a resource with hosts: gets one row per host. This is the
same record install consults to decide whether a resource can be skipped.`,
	Example: `  somcli status
  somcli status --workdir /opt/somwork`,
	Run: func(cmd *cobra.Command, args []string) {
		store := state.Load()
		records := store.Records()

		if len(records) == 0 {
			fmt.Printf("没有任何安装记录（状态文件：%s）\n", state.Path())
			return
		}

		out := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(out, "NAME\tVERSION\tTARGET\tMETHOD\tUPDATED")
		for _, rec := range records {
			method := rec.Method
			if method == "" {
				method = "script"
			}
			fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n",
				rec.Name, rec.Version, rec.Target(), method, rec.UpdatedAt.Format("2006-01-02 15:04:05"))
		}
		out.Flush()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
