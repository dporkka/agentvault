const { getDefaultConfig } = require('expo/metro-config');
const path = require('path');

const projectRoot = __dirname;
const contractRoot = path.resolve(projectRoot, '../../packages/contract');

const config = getDefaultConfig(projectRoot);

// Watch the shared contract package
config.watchFolders = [contractRoot];

// Force Metro to resolve `@agentvault/contract` and bare `agentvault/contract`
// imports to the shared package without falling back to node_modules.
config.resolver.nodeModulesPaths = [
  path.resolve(projectRoot, 'node_modules'),
  contractRoot,
];
// Metro doesn't honor tsconfig 'paths' aliases for monorepo packages.
// Disable hierarchical lookup so `@agentvault/contract` resolves to the
// bare file: dep declared in package.json instead of being shadowed by
// a parent node_modules folder.
config.resolver.disableHierarchicalLookup = true;

module.exports = config;
