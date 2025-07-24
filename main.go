package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"dm-agent/helpers"
	"dm-agent/ui"
)

func main() {

	ui.Println(ui.Blue, strings.Repeat("=", 80))

	modelRunnerBaseUrl := os.Getenv("MODEL_RUNNER_BASE_URL")

	if modelRunnerBaseUrl == "" {
		panic("MODEL_RUNNER_BASE_URL environment variable is not set")
	}
	ui.Println(ui.Blue, "Model Runner Base URL:", modelRunnerBaseUrl)

	modelRunnerChatModel := os.Getenv("MODEL_RUNNER_CHAT_MODEL")

	if modelRunnerChatModel == "" {
		panic("MODEL_RUNNER_CHAT_MODEL environment variable is not set")
	}

	ui.Println(ui.Blue, "Model Runner Chat Model:", modelRunnerChatModel)

	modelRunnerToolsModel := os.Getenv("MODEL_RUNNER_TOOLS_MODEL")
	if modelRunnerToolsModel == "" {
		panic("MODEL_RUNNER_TOOLS_MODEL environment variable is not set")
	}

	ui.Println(ui.Blue, "Model Runner Tools Model:", modelRunnerToolsModel)

	ui.Println(ui.Blue, strings.Repeat("=", 80))

	systemInstructions, err := helpers.ReadTextFile("instructions.md")
	if err != nil {
		panic(err)
	}
	// NOTE: try without this
	systemToolsInstructions := ` 
	Your job is to understand the user prompt and decide if you need to use tools to run external commands.
	Ignore all things not related to the usage of a tool
	`

	systemToolsInstructionsForChat := ` 
	If you detect that the user prompt is related to a tool, 
	ignore this part and focus on the other parts.
	`

	characterSheet, err := helpers.ReadTextFile("character_sheet.md")
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	clientEngine := openai.NewClient(
		option.WithBaseURL(modelRunnerBaseUrl),
		option.WithAPIKey(""),
	)

	// --- TOOLS ---

	mcpClient, err := client.NewStreamableHttpClient(
		os.Getenv("MCP_HOST_URL"), // Use environment variable for MCP host
	)
	//defer mcpClient.Close()
	if err != nil {
		fmt.Println("🔴 Failed to create MCP client:", err)
		panic(err)
	}

	// Start the connection to the server
	err = mcpClient.Start(ctx)
	if err != nil {
		fmt.Println("🔴 Failed to start MCP client:", err)
		panic(err)
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "bob",
		Version: "0.0.0",
	}

	result, err := mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		fmt.Println("🔴 Failed to initialize MCP client:", err)
		panic(err)
	}
	fmt.Println("Streamable HTTP client connected & initialized with server!", result)

	toolsRequest := mcp.ListToolsRequest{}
	mcpTools, err := mcpClient.ListTools(ctx, toolsRequest)
	if err != nil {
		panic(err)
	}
	fmt.Println("Available Tools:")
	for _, tool := range mcpTools.Tools {
		fmt.Printf("🛠️ Tool: %s\n", tool.Name)
		fmt.Printf("  Description: %s\n", tool.Description)
	}

	openAITools := ConvertMCPToolsToOpenAITools(mcpTools)

	// --- TOOLS ---

	// Tools Completion parameters
	toolsCompletionParams := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{},
		//ParallelToolCalls: openai.Bool(true),
		ParallelToolCalls: openai.Bool(false),
		Tools:             openAITools,
		Model:             modelRunnerToolsModel,
		Temperature:       openai.Opt(0.0),
	}

	// Chat Completion parameters
	chatCompletionParams := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("CONTEXT:\n" + characterSheet),
			openai.SystemMessage(systemInstructions),
			//openai.UserMessage(userQuestion), // NOTE: to be removed
		},
		Model:       modelRunnerChatModel,
		Temperature: openai.Opt(0.5),
	}

	type PromptConfig struct {
		StartingMessage            string
		ExplanationMessage         string
		PromptTitle                string
		ThinkingPrompt             string
		InterruptInstructions      string
		CompletionInterruptMessage string
		GoodbyeMessage             string
	}
	promptConfig := PromptConfig{
		StartingMessage:       "😄 I'm the Dungeon Master",
		ExplanationMessage:    "Ask me anything about me. Type '/bye' to quit or Ctrl+C to interrupt responses.",
		PromptTitle:           "✋ Query",
		ThinkingPrompt:        "⏳",
		InterruptInstructions: "(Press Ctrl+C to interrupt)",
		//CompletionInterruptMessage: "⚠️ Response was interrupted\n",
		GoodbyeMessage: "🐺 Bye!",
	}

	//reader := bufio.NewScanner(os.Stdin)
	fmt.Println(promptConfig.StartingMessage)
	fmt.Println(promptConfig.ExplanationMessage)

	for {
		fmt.Print(promptConfig.ThinkingPrompt)
		fmt.Println(promptConfig.InterruptInstructions)

		var userInput string

		form := huh.NewForm(
			huh.NewGroup(
				huh.NewText().
					Title(promptConfig.PromptTitle).
					Placeholder("Type your question here...").
					Value(&userInput).
					ExternalEditor(false),
			),
		)

		// Run the form
		err := form.Run()
		if err != nil {
			// TODO: handle error
		}

		// Trim whitespace
		userInput = strings.TrimSpace(userInput)

		// Check for empty input
		if userInput == "" {
			continue
		}

		// Check for /bye command
		if userInput == "/bye" {
			fmt.Println(promptConfig.GoodbyeMessage)
			break
		}

		// Completions here...
		// TOOLS DETECTION:
		fmt.Println("🚀 Starting tools detection...")

		// IMPORTANT: do not forget to set the user question in the params
		toolsCompletionParams.Messages = []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemToolsInstructions),
			openai.UserMessage(userInput),
		}

		fmt.Println("⏳ Running tools completion...")
		// Make initial Tool completion request
		// TOOLS COMPLETION:
		completion, err := clientEngine.Chat.Completions.New(ctx, toolsCompletionParams)
		if err != nil {
			fmt.Printf("😡 Tools completion error: %v\n", err)
			continue
		}

		fmt.Println("🛠️ Tools completion received")
		detectedToolCalls := completion.Choices[0].Message.ToolCalls

		firstCompletionResult := ""
		// Return early if there are no tool calls
		if len(detectedToolCalls) == 0 {
			fmt.Println("✋ No function call")
			fmt.Println()
			//continue
		} else {
			// TOOL CALLS:

			for _, toolCall := range detectedToolCalls {

				// Display the detected tool call
				fmt.Println("💡 tool detection:", toolCall.Function.Name, toolCall.Function.Arguments)

				// Parse the tool arguments from JSON string
				var args map[string]any
				args, _ = JsonStringToMap(toolCall.Function.Arguments)

				// NOTE: Call the MCP tool with the arguments
				request := mcp.CallToolRequest{}
				request.Params.Name = toolCall.Function.Name
				request.Params.Arguments = args

				// NOTE: Call the tool using the MCP client
				toolResponse, err := mcpClient.CallTool(ctx, request)

				if err != nil {
					fmt.Println("🔴 Error calling tool:", err)
					continue
				} else {
					if toolResponse != nil && len(toolResponse.Content) > 0 {
						// TODO: test if the content is a TextContent
						result := toolResponse.Content[0].(mcp.TextContent).Text
						fmt.Printf("✅ Tool %s executed successfully, result: %s\n", toolCall.Function.Name, result)
						firstCompletionResult += result + "\n"
					}
				}

				// switch toolCall.Function.Name {
				// // TOOL 1:
				// case "fetch_content":
				// 	// what to do with the result?

				// // TOOL 2:
				// case "search":
				// 	// what to do with the result?

				// default:
				// 	fmt.Println("❌ Error: unknown tool", toolCall.Function.Name)

				// }

			}

			fmt.Println("🎉 Tools calls executed!")
		}

		fmt.Println("🤖 Starting chat completion...")
		fmt.Println(strings.Repeat("=", 80))

		// CHAT COMPLETION:
		chatCompletionParams.Messages = append(
			chatCompletionParams.Messages,
			openai.SystemMessage(firstCompletionResult), // NOTE: could be empty
			openai.SystemMessage(systemToolsInstructionsForChat),
			openai.UserMessage(userInput),
		)

		stream := clientEngine.Chat.Completions.NewStreaming(ctx, chatCompletionParams)

		for stream.Next() {
			chunk := stream.Current()
			// Stream each chunk as it arrives
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				fmt.Print(chunk.Choices[0].Delta.Content)
			}
		}

		if err := stream.Err(); err != nil {
			fmt.Printf("😡 Stream error: %v\n", err)
		}

		fmt.Println("\n" + strings.Repeat("=", 80))
		fmt.Println()

		fmt.Println() // Add spacing between interactions
	}

}

func JsonStringToMap(jsonString string) (map[string]any, error) {
	var result map[string]any
	err := json.Unmarshal([]byte(jsonString), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func ConvertMCPToolsToOpenAITools(tools *mcp.ListToolsResult) []openai.ChatCompletionToolParam {
	openAITools := make([]openai.ChatCompletionToolParam, len(tools.Tools))
	for i, tool := range tools.Tools {

		openAITools[i] = openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters: openai.FunctionParameters{
					"type":       "object",
					"properties": tool.InputSchema.Properties,
					"required":   tool.InputSchema.Required,
				},
			},
		}
	}
	return openAITools
}
