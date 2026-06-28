module.exports = {
  root: true,
  extends: ['expo', 'plugin:prettier/recommended'],
  plugins: ['prettier'],
  rules: {
    'prettier/prettier': 'error',
  },
  ignorePatterns: ['dist/', '.expo/', 'node_modules/'],
};
