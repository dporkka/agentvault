import React from 'react';
import { StatusBar } from 'expo-status-bar';
import { NavigationContainer } from '@react-navigation/native';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { createStackNavigator } from '@react-navigation/stack';
import { Text, StyleSheet } from 'react-native';
import Ionicons from '@expo/vector-icons/Ionicons';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { colors, spacing, typography, layout } from './src/theme';

import { SettingsProvider } from './src/context/SettingsContext';
import ErrorBoundary from './src/components/ErrorBoundary';
import { useAutoSync } from './src/hooks/useAutoSync';
import type { RootStackParamList, RootTabParamList } from './src/navigation/types';
import HomeScreen from './src/screens/HomeScreen';
import CaptureScreen from './src/screens/CaptureScreen';
import InboxScreen from './src/screens/InboxScreen';
import SearchScreen from './src/screens/SearchScreen';
import SettingsScreen from './src/screens/SettingsScreen';
import NoteDetailScreen from './src/screens/NoteDetailScreen';

const Tab = createBottomTabNavigator<RootTabParamList>();
const Stack = createStackNavigator<RootStackParamList>();

function TabLabel({ focused, label }: { focused: boolean; label: string }) {
  return <Text style={[styles.tabLabel, focused && styles.tabLabelFocused]}>{label}</Text>;
}

function MainTabs() {
  return (
    <Tab.Navigator
      screenOptions={{
        headerShown: false,
        tabBarStyle: styles.tabBar,
        tabBarActiveTintColor: colors.accent,
        tabBarInactiveTintColor: colors.textMuted,
      }}
    >
      <Tab.Screen
        name="Home"
        component={HomeScreen}
        options={{
          tabBarLabel: ({ focused }) => <TabLabel focused={focused} label="Home" />,
          tabBarAccessibilityLabel: 'Home',
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="home-outline" size={size} color={color} />
          ),
        }}
      />
      <Tab.Screen
        name="Capture"
        component={CaptureScreen}
        options={{
          tabBarLabel: ({ focused }) => <TabLabel focused={focused} label="Capture" />,
          tabBarAccessibilityLabel: 'Capture',
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="add-circle-outline" size={size} color={color} />
          ),
        }}
      />
      <Tab.Screen
        name="Inbox"
        component={InboxScreen}
        options={{
          tabBarLabel: ({ focused }) => <TabLabel focused={focused} label="Inbox" />,
          tabBarAccessibilityLabel: 'Inbox',
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="file-tray-full-outline" size={size} color={color} />
          ),
        }}
      />
      <Tab.Screen
        name="Search"
        component={SearchScreen}
        options={{
          tabBarLabel: ({ focused }) => <TabLabel focused={focused} label="Search" />,
          tabBarAccessibilityLabel: 'Search',
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="search-outline" size={size} color={color} />
          ),
        }}
      />
      <Tab.Screen
        name="Settings"
        component={SettingsScreen}
        options={{
          tabBarLabel: ({ focused }) => <TabLabel focused={focused} label="Settings" />,
          tabBarAccessibilityLabel: 'Settings',
          tabBarIcon: ({ color, size }) => (
            <Ionicons name="settings-outline" size={size} color={color} />
          ),
        }}
      />
    </Tab.Navigator>
  );
}

export default function App() {
  useAutoSync();
  return (
    <SafeAreaProvider>
      <ErrorBoundary>
        <SettingsProvider>
          <NavigationContainer>
            <StatusBar style="light" />
            <Stack.Navigator screenOptions={{ headerShown: false }}>
              <Stack.Screen name="MainTabs" component={MainTabs} />
              <Stack.Screen
                name="NoteDetail"
                component={NoteDetailScreen}
                options={{
                  cardStyle: { backgroundColor: colors.bgPrimary },
                }}
              />
            </Stack.Navigator>
          </NavigationContainer>
        </SettingsProvider>
      </ErrorBoundary>
      <SettingsProvider>
        <NavigationContainer>
          <StatusBar style="light" />
          <Tab.Navigator
            screenOptions={{
              headerShown: false,
              tabBarStyle: styles.tabBar,
              tabBarActiveTintColor: '#4f7cff',
              tabBarInactiveTintColor: '#6b7280',
            }}
          >
            <Tab.Screen
              name="Home"
              component={HomeScreen}
              options={{
                tabBarLabel: ({ focused }) => <TabLabel focused={focused} label="Home" />,
                tabBarIcon: ({ color }) => (
                  <Text style={[styles.icon, { color }]}>H</Text>
                ),
              }}
            />
            <Tab.Screen
              name="Capture"
              component={CaptureScreen}
              options={{
                tabBarLabel: ({ focused }) => <TabLabel focused={focused} label="Capture" />,
                tabBarIcon: ({ color }) => (
                  <Text style={[styles.icon, { color }]}>+</Text>
                ),
              }}
            />
            <Tab.Screen
              name="Inbox"
              component={InboxScreen}
              options={{
                tabBarLabel: ({ focused }) => <TabLabel focused={focused} label="Inbox" />,
                tabBarIcon: ({ color }) => (
                  <Text style={[styles.icon, { color }]}>I</Text>
                ),
              }}
            />
            <Tab.Screen
              name="Search"
              component={SearchScreen}
              options={{
                tabBarLabel: ({ focused }) => <TabLabel focused={focused} label="Search" />,
                tabBarIcon: ({ color }) => (
                  <Text style={[styles.icon, { color }]}>Q</Text>
                ),
              }}
            />
            <Tab.Screen
              name="Settings"
              component={SettingsScreen}
              options={{
                tabBarLabel: ({ focused }) => <TabLabel focused={focused} label="Settings" />,
                tabBarIcon: ({ color }) => (
                  <Text style={[styles.icon, { color }]}>S</Text>
                ),
              }}
            />
          </Tab.Navigator>
        </NavigationContainer>
      </SettingsProvider>
    </SafeAreaProvider>
  );
}

const styles = StyleSheet.create({
  tabBar: {
    backgroundColor: colors.bgSecondary,
    borderTopWidth: 1,
    borderTopColor: colors.borderSubtle,
    paddingBottom: spacing.xs,
    paddingTop: spacing.xs,
    height: layout.tabBarHeight,
  },
  tabLabel: {
    fontSize: 10,
    fontWeight: typography.weights.medium,
    color: colors.textMuted,
  },
  tabLabelFocused: {
    color: colors.accent,
    fontWeight: typography.weights.bold,
  },
});
