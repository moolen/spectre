/**
 * Creating a sidebar enables you to:
 - create an ordered group of docs
 - render a sidebar for each doc of that group
 - provide next/previous navigation

 The sidebars can be generated from the filesystem, or explicitly defined here.

 Create as many sidebars as you want.
 */

// @ts-check

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  // Main documentation sidebar
  docsSidebar: [
    'intro',

    {
      type: 'category',
      label: 'Getting Started',
      link: {
        type: 'doc',
        id: 'getting-started/index',
      },
      collapsed: false,
      items: [
        'getting-started/quick-start',
        'getting-started/demo-mode',
      ],
    },

    {
      type: 'category',
      label: 'Installation',
      link: {
        type: 'doc',
        id: 'installation/index',
      },
      items: [
        'installation/helm',
        'installation/local-development',
      ],
    },

    {
      type: 'category',
      label: 'Configuration',
      link: {
        type: 'doc',
        id: 'configuration/index',
      },
      items: [
        'configuration/watcher-config',
        'configuration/storage-settings',
        'configuration/mcp-configuration',
      ],
    },

    {
      type: 'category',
      label: 'User Guide',
      link: {
        type: 'doc',
        id: 'user-guide/index',
      },
      items: [
        'user-guide/ui-overview',
        'user-guide/querying-events',
        'user-guide/filtering-events',
        'user-guide/timeline-visualization',
      ],
    },

    {
      type: 'category',
      label: 'Use Cases',
      link: {
        type: 'doc',
        id: 'use-cases/index',
      },
      items: [
        'use-cases/incident-investigation',
        'use-cases/post-mortem-analysis',
        'use-cases/deployment-tracking',
      ],
    },

    {
      type: 'category',
      label: 'MCP Integration',
      link: {
        type: 'doc',
        id: 'mcp-integration/index',
      },
      items: [
        'mcp-integration/getting-started',
        {
          type: 'category',
          label: 'Tools Reference',
          items: [
            'mcp-integration/tools-reference/cluster-health',
            'mcp-integration/tools-reference/resource-changes',
            'mcp-integration/tools-reference/resource-timeline',
          ],
        },
        {
          type: 'category',
          label: 'Prompts Reference',
          items: [
            'mcp-integration/prompts-reference/post-mortem',
            'mcp-integration/prompts-reference/live-incident',
          ],
        },
        'mcp-integration/claude-integration',
        'mcp-integration/examples',
      ],
    },

    {
      type: 'category',
      label: 'Architecture',
      link: {
        type: 'doc',
        id: 'architecture/index',
      },
      items: [
        'architecture/overview',
        'architecture/storage-design',
        'architecture/block-format',
        'architecture/indexing-strategy',
        'architecture/compression',
        'architecture/query-execution',
        'architecture/data-flow',
      ],
    },

    // Hidden sections (not ready yet)
    // {
    //   type: 'category',
    //   label: 'API Reference',
    //   link: {
    //     type: 'doc',
    //     id: 'api/index',
    //   },
    //   items: [
    //     {
    //       type: 'category',
    //       label: 'REST API',
    //       items: [
    //         'api/rest-api/search',
    //         'api/rest-api/metadata',
    //         'api/rest-api/export',
    //         'api/rest-api/import',
    //       ],
    //     },
    //   ],
    // },
    //
    // {
    //   type: 'category',
    //   label: 'Operations',
    //   link: {
    //     type: 'doc',
    //     id: 'operations/index',
    //   },
    //   items: [
    //     'operations/deployment',
    //     'operations/monitoring',
    //     'operations/troubleshooting',
    //     'operations/storage-management',
    //     'operations/performance-tuning',
    //     'operations/backup-recovery',
    //   ],
    // },
    //
    // {
    //   type: 'category',
    //   label: 'Development',
    //   link: {
    //     type: 'doc',
    //     id: 'development/index',
    //   },
    //   items: [
    //     'development/contributing',
    //     'development/development-setup',
    //     'development/testing',
    //     'development/building',
    //     'development/code-structure',
    //     'development/release-process',
    //   ],
    // },
    //
    // {
    //   type: 'category',
    //   label: 'Reference',
    //   items: [
    //     'reference/cli-commands',
    //     'reference/helm-values',
    //     'reference/api-spec',
    //     'reference/glossary',
    //   ],
    // },
  ],
};

module.exports = sidebars;
