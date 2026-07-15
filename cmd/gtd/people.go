package main

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"gtd/internal/domain"
)

var peopleCmd = &cobra.Command{
	Use:   "people",
	Short: "Manage people",
	Long: `Manage people (delegates) associated with tasks.
People are used to track 'waiting' tasks.`,
}

var peopleAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new person",
	Long: `Adds a new person to the workspace with the specified name. Returns the JSON person representation.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		person := &domain.Person{
			ID:   uuid.New().String(),
			Name: name,
		}

		if err := appCtx.personRepo.Save(person); err != nil {
			return fmt.Errorf("save person: %w", err)
		}

		if err := appCtx.syncEngine.SyncPerson(context.Background(), person); err != nil {
			return fmt.Errorf("sync person: %w", err)
		}

		printSuccess(person)
		return nil
	},
}

var peopleDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a person",
	Long: `Soft-deletes a person by ID.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		person, err := appCtx.personRepo.Get(id)
		if err != nil {
			return fmt.Errorf("person not found: %w", err)
		}

		now := time.Now()
		person.SoftDelete(now)

		if err := appCtx.personRepo.Save(person); err != nil {
			return fmt.Errorf("save person: %w", err)
		}

		if err := appCtx.syncEngine.SyncPerson(context.Background(), person); err != nil {
			return fmt.Errorf("sync person: %w", err)
		}

		printSuccess(person)
		return nil
	},
}

var peopleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List people",
	Long: `Lists all active person IDs. Defaults to JSON list output.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCtx, err := getAppContext()
		if err != nil {
			return err
		}
		defer appCtx.cleanup()

		people, err := appCtx.personRepo.List()
		if err != nil {
			return fmt.Errorf("list people: %w", err)
		}

		activePeople := make([]*domain.Person, 0)
		for _, p := range people {
			if p.DeletedAt == nil {
				activePeople = append(activePeople, p)
			}
		}

		printSuccess(activePeople)
		return nil
	},
}

func init() {
	peopleCmd.AddCommand(peopleAddCmd)
	peopleCmd.AddCommand(peopleDeleteCmd)
	peopleCmd.AddCommand(peopleListCmd)

	rootCmd.AddCommand(peopleCmd)
}
