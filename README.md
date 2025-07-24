# Demo Simple DM Agent with Docker MCP Gateway

## Start the application

**With Docker Compose**:
```bash
docker compose up --build -d
docker attach $(docker compose ps -q dm-agent)
```
> `docker compose down` to stop the application.


**From a container**:
```bash
MODEL_RUNNER_BASE_URL=http://model-runner.docker.internal/engines/llama.cpp/v1 \
MODEL_RUNNER_CHAT_MODEL=hf.co/menlo/lucy-128k-gguf:q4_k_m \
MODEL_RUNNER_TOOLS_MODEL=hf.co/menlo/lucy-128k-gguf:q4_k_m \
go run main.go
```


**From a local machine**:
```bash
MODEL_RUNNER_BASE_URL=http://localhost:12434/engines/llama.cpp/v1 \
MODEL_RUNNER_CHAT_MODEL=hf.co/menlo/lucy-128k-gguf:q4_k_m \
MODEL_RUNNER_TOOLS_MODEL=hf.co/menlo/lucy-128k-gguf:q4_k_m \
go run main.go
```

## Talk to the DM

```raw
Fetch and parse content from https://raw.githubusercontent.com/Compose-and-Dragons/data/refs/heads/main/chronicles.md and summarize the content
```

```raw
Fetch and parse content from https://raw.githubusercontent.com/Compose-and-Dragons/data/refs/heads/main/chronicles.md and tell me who is KeegOrg
```



## Information

### Devcontainer
This project is designed to run in a [Devcontainer](https://code.visualstudio.com/docs/devcontainers/containers) environment.
You can use the provided `.devcontainer` configuration in the `snippets` directory to set up the environment in Visual Studio Code or any compatible IDE.
