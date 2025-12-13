---
title: Demo Mode
description: Try Spectre with embedded sample data
keywords: [demo, sample data, tutorial]
---

# Demo Mode

Try Spectre without deploying to a Kubernetes cluster using the built-in demo mode with sample data.

## What is Demo Mode?

Demo mode runs Spectre with pre-loaded sample events, allowing you to:
- Explore the UI without a Kubernetes cluster
- Learn how to use Spectre before deploying
- Test queries and filters
- Demo Spectre to your team

## Running Demo Mode

### Using Docker

The easiest way to run demo mode is with Docker:

```bash
docker run -it -p 8080:8080 ghcr.io/moolen/spectre:master --demo
```

### Using Local Binary

If you have built Spectre from source:

```bash
./spectre server --demo
```

## Accessing the Demo

Once running, open your browser to:

```
http://localhost:8080
```

You'll see the Spectre UI with pre-loaded sample events.

## What's Included in Demo Data?

The demo data includes:
- Sample Pod events (create, update, delete)
- Deployment rollout scenarios
- Failed pod examples
- Status transitions
- Approximately 1 hour of simulated events

## Exploring Demo Mode

Try these queries to explore the demo data:

### View All Events
- Keep the default time range
- No filters applied
- See all captured events

### Filter by Kind
- Select "Pod" from the Kind filter
- See only Pod events

### Filter by Namespace
- Select a specific namespace
- See events for that namespace only

### Time Range Queries
- Adjust the time range slider
- Zoom in to specific time periods

## Limitations

Demo mode has some limitations:
- **Read-only** - No new events are captured
- **Fixed dataset** - Same events every time
- **No MCP** - AI-assisted analysis not available in demo mode
- **In-memory only** - Data is not persisted

## Next Steps

After exploring demo mode:

1. **Install in your cluster** - Follow the [Quick Start](./quick-start) guide
2. **Learn about configuration** - See [Configuration](../configuration)
3. **Set up MCP** - Enable AI analysis with [MCP Integration](../mcp-integration)

<!-- Source: /home/moritz/dev/spectre/README.md lines 62-70 -->
