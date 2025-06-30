package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/sync/semaphore"
)

// GraphQL queries for batch operations
const (
	repositoryTreeQuery = `
	query($owner: String!, $name: String!, $expression: String!) {
		repository(owner: $owner, name: $name) {
			object(expression: $expression) {
				... on Tree {
					entries {
						name
						path
						type
						object {
							... on Blob {
								text
								byteSize
							}
						}
					}
				}
			}
		}
	}`

	multipleRepositoriesQuery = `
	query($org: String!, $first: Int!, $after: String) {
		organization(login: $org) {
			repositories(first: $first, after: $after, orderBy: {field: UPDATED_AT, direction: DESC}) {
				nodes {
					id
					name
					nameWithOwner
					description
					primaryLanguage {
						name
					}
					url
					createdAt
					updatedAt
					pushedAt
					diskUsage
					isPrivate
					isFork
					isArchived
					issues {
						totalCount
					}
					defaultBranchRef {
						name
					}
				}
				pageInfo {
					hasNextPage
					endCursor
				}
			}
		}
	}`
)

type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

type GraphQLClient struct {
	httpClient  *http.Client
	token       string
	semaphore   *semaphore.Weighted
	rateLimiter *time.Ticker
}

func NewGraphQLClient(token string, concurrency int) *GraphQLClient {
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 50,
			IdleConnTimeout:     90 * time.Second,
			MaxConnsPerHost:     20,
		},
		Timeout: 30 * time.Second,
	}

	rateLimit := time.Millisecond * 50 // GraphQL allows more aggressive rate limiting
	if token == "" {
		rateLimit = time.Millisecond * 500
	}

	return &GraphQLClient{
		httpClient:  httpClient,
		token:       token,
		semaphore:   semaphore.NewWeighted(int64(concurrency)),
		rateLimiter: time.NewTicker(rateLimit),
	}
}

func (c *GraphQLClient) executeQuery(ctx context.Context, query string, variables map[string]interface{}) (*GraphQLResponse, error) {
	if err := c.semaphore.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer c.semaphore.Release(1)

	<-c.rateLimiter.C

	payload := map[string]interface{}{
		"query":     query,
		"variables": variables,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.github.com/graphql", strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GraphQL request failed: %w", err)
	}
	defer resp.Body.Close()

	var graphqlResp GraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&graphqlResp); err != nil {
		return nil, fmt.Errorf("failed to decode GraphQL response: %w", err)
	}

	if len(graphqlResp.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %v", graphqlResp.Errors)
	}

	return &graphqlResp, nil
}

// GetRepositoryTreeWithContent fetches both tree structure and file contents in a single GraphQL query
func (c *GraphQLClient) GetRepositoryTreeWithContent(ctx context.Context, owner, repo, branch string) (map[string]string, error) {
	variables := map[string]interface{}{
		"owner":      owner,
		"name":       repo,
		"expression": branch + ":",
	}

	resp, err := c.executeQuery(ctx, repositoryTreeQuery, variables)
	if err != nil {
		return nil, err
	}

	// Parse the response to extract file contents
	var data struct {
		Repository struct {
			Object struct {
				Entries []struct {
					Name string `json:"name"`
					Path string `json:"path"`
					Type string `json:"type"`
					Object struct {
						Text     string `json:"text"`
						ByteSize int    `json:"byteSize"`
					} `json:"object"`
				} `json:"entries"`
			} `json:"object"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	fileContents := make(map[string]string)
	for _, entry := range data.Repository.Object.Entries {
		if entry.Type == "blob" && isTerraformFile(entry.Path) {
			fileContents[entry.Path] = entry.Object.Text
		}
	}

	return fileContents, nil
}

func isTerraformFile(path string) bool {
	return strings.HasSuffix(path, ".tf") ||
		strings.HasSuffix(path, ".tfvars") ||
		strings.HasSuffix(path, ".hcl")
}

// BatchGetRepositories fetches multiple repositories in a single GraphQL query
func (c *GraphQLClient) BatchGetRepositories(ctx context.Context, org string, limit int) ([]Repository, error) {
	var allRepos []Repository
	var cursor *string

	for {
		variables := map[string]interface{}{
			"org":   org,
			"first": limit,
		}
		if cursor != nil {
			variables["after"] = *cursor
		}

		resp, err := c.executeQuery(ctx, multipleRepositoriesQuery, variables)
		if err != nil {
			return nil, err
		}

		var data struct {
			Organization struct {
				Repositories struct {
					Nodes []struct {
						ID          string    `json:"id"`
						Name        string    `json:"name"`
						NameWithOwner string  `json:"nameWithOwner"`
						Description string    `json:"description"`
						PrimaryLanguage *struct {
							Name string `json:"name"`
						} `json:"primaryLanguage"`
						URL       string    `json:"url"`
						CreatedAt time.Time `json:"createdAt"`
						UpdatedAt time.Time `json:"updatedAt"`
						PushedAt  time.Time `json:"pushedAt"`
						DiskUsage int       `json:"diskUsage"`
						IsPrivate bool      `json:"isPrivate"`
						IsFork    bool      `json:"isFork"`
						IsArchived bool     `json:"isArchived"`
						Issues    struct {
							TotalCount int `json:"totalCount"`
						} `json:"issues"`
						DefaultBranchRef struct {
							Name string `json:"name"`
						} `json:"defaultBranchRef"`
					} `json:"nodes"`
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
				} `json:"repositories"`
			} `json:"organization"`
		}

		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to parse repositories response: %w", err)
		}

		for _, repo := range data.Organization.Repositories.Nodes {
			language := ""
			if repo.PrimaryLanguage != nil {
				language = repo.PrimaryLanguage.Name
			}

			allRepos = append(allRepos, Repository{
				ID:            0, // GraphQL doesn't return numeric ID
				FullName:      repo.NameWithOwner,
				Name:          repo.Name,
				Organization:  org,
				Description:   repo.Description,
				Language:      language,
				HTMLURL:       repo.URL,
				CreatedAt:     repo.CreatedAt,
				UpdatedAt:     repo.UpdatedAt,
				PushedAt:      repo.PushedAt,
				Size:          repo.DiskUsage,
				IsPrivate:     repo.IsPrivate,
				IsFork:        repo.IsFork,
				IsArchived:    repo.IsArchived,
				OpenIssues:    repo.Issues.TotalCount,
				DefaultBranch: repo.DefaultBranchRef.Name,
			})
		}

		if !data.Organization.Repositories.PageInfo.HasNextPage {
			break
		}
		cursor = &data.Organization.Repositories.PageInfo.EndCursor
	}

	return allRepos, nil
}