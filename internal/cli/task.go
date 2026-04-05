package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/emirtuncer/codesight/internal/markdown"
	"github.com/emirtuncer/codesight/internal/tasks"
	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks in the .codesight index",
}

// --- task create ---

var (
	taskCreateProject string
	taskCreateUrgency string
)

var taskCreateCmd = &cobra.Command{
	Use:   "create <title>",
	Short: "Create a new task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		codesightDir, err := findCodesightDir()
		if err != nil {
			return err
		}

		project := taskCreateProject
		if project == "" {
			project = defaultProjectName(codesightDir)
		}

		urgency := taskCreateUrgency
		if urgency == "" {
			urgency = markdown.UrgencyMedium
		}

		task, err := tasks.Create(codesightDir, tasks.CreateOpts{
			Title:   args[0],
			Project: project,
			Urgency: urgency,
		})
		if err != nil {
			return fmt.Errorf("create task: %w", err)
		}

		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(task)
		}

		fmt.Printf("Created task %s: %s [%s]\n", task.ID, task.Title, task.Urgency)
		return nil
	},
}

// --- task list ---

var (
	taskListProject  string
	taskListStatus   string
	taskListUrgency  string
	taskListAssigned string
)

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		codesightDir, err := findCodesightDir()
		if err != nil {
			return err
		}

		list, err := tasks.List(codesightDir, tasks.TaskFilter{
			Project:    taskListProject,
			Status:     taskListStatus,
			Urgency:    taskListUrgency,
			AssignedTo: taskListAssigned,
		})
		if err != nil {
			return fmt.Errorf("list tasks: %w", err)
		}

		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(list)
		}

		if len(list) == 0 {
			fmt.Println("No tasks found.")
			return nil
		}

		// Print table header.
		fmt.Printf("%-12s %-10s %-10s %-15s %s\n", "ID", "STATUS", "URGENCY", "PROJECT", "TITLE")
		fmt.Printf("%-12s %-10s %-10s %-15s %s\n",
			"------------", "----------", "----------", "---------------", "-----")
		for _, t := range list {
			project := t.Project
			if len(project) > 15 {
				project = project[:12] + "..."
			}
			fmt.Printf("%-12s %-10s %-10s %-15s %s\n",
				t.ID, t.Status, t.Urgency, project, t.Title)
		}
		return nil
	},
}

// --- task update ---

var (
	taskUpdateProject  string
	taskUpdateStatus   string
	taskUpdateUrgency  string
	taskUpdateAssigned string
)

var taskUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a task's status, urgency, or assignment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		codesightDir, err := findCodesightDir()
		if err != nil {
			return err
		}

		project := taskUpdateProject
		if project == "" {
			project = defaultProjectName(codesightDir)
		}

		if err := tasks.Update(codesightDir, project, args[0], tasks.TaskUpdates{
			Status:     taskUpdateStatus,
			Urgency:    taskUpdateUrgency,
			AssignedTo: taskUpdateAssigned,
		}); err != nil {
			return fmt.Errorf("update task: %w", err)
		}

		fmt.Printf("Updated task %s\n", args[0])
		return nil
	},
}

func init() {
	// create flags
	taskCreateCmd.Flags().StringVar(&taskCreateProject, "project", "", "project name")
	taskCreateCmd.Flags().StringVar(&taskCreateUrgency, "urgency", "", "urgency: critical, urgent, medium, low (default: medium)")

	// list flags
	taskListCmd.Flags().StringVar(&taskListProject, "project", "", "filter by project")
	taskListCmd.Flags().StringVar(&taskListStatus, "status", "", "filter by status")
	taskListCmd.Flags().StringVar(&taskListUrgency, "urgency", "", "filter by urgency")
	taskListCmd.Flags().StringVar(&taskListAssigned, "assigned", "", "filter by assigned-to")

	// update flags
	taskUpdateCmd.Flags().StringVar(&taskUpdateProject, "project", "", "project name (required)")
	taskUpdateCmd.Flags().StringVar(&taskUpdateStatus, "status", "", "new status")
	taskUpdateCmd.Flags().StringVar(&taskUpdateUrgency, "urgency", "", "new urgency")
	taskUpdateCmd.Flags().StringVar(&taskUpdateAssigned, "assign", "", "assign to a person")

	taskCmd.AddCommand(taskCreateCmd, taskListCmd, taskUpdateCmd)
	rootCmd.AddCommand(taskCmd)
}
