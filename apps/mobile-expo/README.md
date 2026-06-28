# AgentVault Mobile

Capture-first Expo/React Native companion for AgentVault.

## Prerequisites

- Node.js 20+
- npm
- An AgentVault server running locally (`agentvault serve` from the core CLI)

## Install

```bash
cd apps/mobile-expo
npm install
```

## Run

```bash
# Start the Expo development server
npm run start

# Run on a specific platform
npm run android
npm run ios
```

## Verify

```bash
# Type-check
npm run typecheck

# Run unit tests
npm test

# Lint
npm run lint

# Check formatting
npm run format:check

# Expo diagnostics
npm run doctor
```

## Build

```bash
# Export an iOS bundle (used in CI)
npx expo export --platform ios

# Build with EAS
npx eas build --profile development
npx eas build --profile production
```

## Project Structure

```
apps/mobile-expo/
├── App.tsx                 # Root navigation (SafeAreaProvider, ErrorBoundary, SettingsProvider)
├── src/
│   ├── api/agentvault.ts   # HTTP client wrapper over @agentvault/contract
│   ├── components/         # Reusable UI components
│   ├── context/            # SettingsContext
│   ├── hooks/              # useCaptures, useConnection, useAutoSync
│   ├── navigation/         # Typed navigation param lists
│   ├── screens/            # Screen components
│   ├── storage/            # AsyncStorage/SecureStore persistence and sync logic
│   ├── theme.ts            # Central theme tokens
│   └── types/index.ts      # Local TypeScript types
```

## Notes

- The auth token is stored in `expo-secure-store` when available; general settings live in AsyncStorage.
- Captures are stored locally first and synced to the server when it is reachable.
- `metro.config.js` disables hierarchical node_modules lookup so the shared `@agentvault/contract` package resolves correctly in the monorepo.
