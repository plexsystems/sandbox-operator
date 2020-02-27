package controller

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/graphrbac/1.6/graphrbac"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	rbacv1 "k8s.io/api/rbac/v1"
)

// AzureSubjects is a client that connects to Azure to get a users ObjectID
type AzureSubjects struct {
	client graphrbac.UsersClient
}

// NewAzureSubjectsClient creates a new client to get azure users
func NewAzureSubjectsClient() (*AzureSubjects, error) {
	const authorizeTimeout = 5 * time.Second

	ctx, cancel := context.WithTimeout(context.TODO(), authorizeTimeout)
	defer cancel()

	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("new authorizer: %w", err)
	}

	graphClient := graphrbac.NewUsersClient(os.Getenv("AZURE_TENANT_ID"))
	graphClient.Authorizer = authorizer

	if _, err := graphClient.List(ctx, ""); err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	azureSubjects := AzureSubjects{
		client: graphClient,
	}

	return &azureSubjects, nil
}

// Subjects gets the ObjectIDs from a list of given emails or user principal names
func (a *AzureSubjects) Subjects(ctx context.Context, users []string) ([]rbacv1.Subject, error) {
	var objectIDs []string

	for _, user := range users {
		filterString := "mail eq '" + user + "' or userPrincipalName eq '" + user + "'"
		userListResultPage, err := a.client.List(ctx, filterString)
		if err != nil {
			return nil, fmt.Errorf("list users: %w", err)
		}

		userListResultPageValues := userListResultPage.Values()
		if userListResultPageValues != nil {
			objectIDs = append(objectIDs, (*userListResultPageValues[0].ObjectID))
		} else {
			log.Printf("%s could not be found\n", user)
		}
	}

	subjects := getSubjects(objectIDs)

	return subjects, nil
}
