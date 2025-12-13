# Spectre Documentation

This directory contains the Docusaurus-based documentation for Spectre.

## Documentation Structure

The documentation is organized into the following sections:

- **Getting Started** - Quick start guides and demo mode
- **Installation** - Helm, Docker, Kubernetes manifests, and local development
- **Configuration** - Watcher, storage, MCP, and environment variables
- **User Guide** - UI overview, querying, filtering, and visualization
- **Use Cases** - Incident investigation, post-mortems, compliance, and deployment tracking
- **MCP Integration** - AI-assisted analysis with Claude (tools, prompts, examples)
- **API Reference** - REST API documentation
- **Architecture** - Storage design, indexing, compression, and query execution
- **Operations** - Deployment, monitoring, troubleshooting, and performance tuning
- **Development** - Contributing, testing, building, and release process
- **Reference** - CLI commands, Helm values, API specs, and glossary

## Local Development

### Prerequisites

- Node.js 18+
- npm or yarn

### Installation

```bash
cd docs
npm install
```

### Running the Development Server

```bash
npm start
```

This command starts a local development server and opens up a browser window. Most changes are reflected live without having to restart the server.

The site will be available at http://localhost:3000/spectre/

### Building

```bash
npm run build
```

This command generates static content into the `build` directory and can be served using any static contents hosting service.

### Testing the Production Build

```bash
npm run serve
```

This serves the production build locally for testing.

## Contributing to Documentation

### Adding a New Page

1. Create a new markdown file in the appropriate section under `docs/`
2. Add frontmatter with title, description, and keywords
3. Update `sidebars.js` if needed to include the new page
4. Test locally with `npm start`

### Editing Existing Pages

Most pages currently contain TODO comments indicating content that needs to be filled in from source files. The source files are referenced at the bottom of each page.

Example workflow:
1. Open a placeholder page (e.g., `docs/architecture/storage-design.md`)
2. Read the referenced source file (e.g., `../docs-backup/ARCHITECTURE.md`)
3. Extract relevant content and rewrite for documentation format
4. Remove TODO comments as sections are completed
5. Test locally to ensure links and formatting work

### Content Sources

Original documentation files are backed up in `/home/moritz/dev/spectre/docs-backup/`:
- `ARCHITECTURE.md` - Architecture section content
- `OPERATIONS.md` - Operations section content
- `BLOCK_FORMAT_REFERENCE.md` - Storage and block format details
- `API.md` - API reference content
- `MCP.md` - MCP integration details

Also reference:
- Main `README.md` in project root
- `chart/values.yaml` for configuration reference
- `Makefile` for CLI commands
- Source code in `internal/`, `cmd/spectre/` for technical details

### Style Guidelines

- Use clear, concise language
- Include code examples with proper syntax highlighting
- Add diagrams or screenshots where helpful
- Use admonitions (:::tip, :::warning, :::info) for important notes
- Link to related pages for context
- Test all code examples

### Markdown Features

Docusaurus supports:
- Standard Markdown
- MDX (React components in Markdown)
- Frontmatter (title, description, keywords)
- Code blocks with syntax highlighting
- Admonitions
- Tabs
- Mermaid diagrams
- Custom React components

See https://docusaurus.io/docs/markdown-features for full documentation.

## Deployment

Documentation is automatically deployed to GitHub Pages when changes are pushed to the `master` branch.

The workflow is defined in `.github/workflows/docs.yml`.

### Manual Deployment

If you need to trigger a deployment manually:
1. Go to GitHub Actions
2. Select "Deploy Documentation to GitHub Pages"
3. Click "Run workflow"

## Project Structure

```
docs/
├── docs/              # Markdown documentation files
│   ├── intro.md
│   ├── getting-started/
│   ├── installation/
│   ├── configuration/
│   ├── user-guide/
│   ├── use-cases/
│   ├── mcp-integration/
│   ├── api/
│   ├── architecture/
│   ├── operations/
│   ├── development/
│   └── reference/
├── src/
│   ├── css/           # Custom CSS
│   ├── pages/         # Custom React pages (landing page)
│   └── components/    # Custom React components
├── static/
│   └── img/          # Images and assets
├── docusaurus.config.js  # Docusaurus configuration
├── sidebars.js       # Sidebar navigation structure
├── package.json      # Dependencies
└── README.md         # This file
```

## Troubleshooting

### Port already in use

If port 3000 is already in use:
```bash
npm start -- --port 3001
```

### Build fails

Clear the cache and rebuild:
```bash
npm run clear
npm run build
```

### Search not working

The search plugin is configured in `docusaurus.config.js`. It automatically indexes content during build. Make sure to rebuild after adding new content.

## Resources

- [Docusaurus Documentation](https://docusaurus.io/)
- [Markdown Guide](https://www.markdownguide.org/)
- [MDX Documentation](https://mdxjs.com/)

## License

Copyright © 2024 Spectre Project. Built with Docusaurus.
