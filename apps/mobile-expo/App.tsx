import React from 'react';
import { StatusBar } from 'expo-status-bar';
import { NavigationContainer } from '@react-navigation/native';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { Text, StyleSheet } from 'react-native';
import { SafeAreaProvider } from 'react-native-safe-area-context';

import { SettingsProvider } from './src/context/SettingsContext';
import HomeScreen from './src/screens/HomeScreen';
import CaptureScreen from './src/screens/CaptureScreen';
import InboxScreen from './src/screens/InboxScreen';
import SearchScreen from './src/screens/SearchScreen';
import SettingsScreen from './src/screens/SettingsScreen';

const Tab = createBottomTabNavigator();

function TabLabel({ focused, label }: { focused: boolean; label: string }) {
  return (
    <Text style={[styles.tabLabel, focused && styles.tabLabelFocused]}>
      {label}
    </Text>
  );
}

export default function App() {
  return (
    <SafeAreaProvider>
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
    backgroundColor: '#1a1d27',
    borderTopWidth: 1,
    borderTopColor: '#252836',
    paddingBottom: 4,
    paddingTop: 4,
    height: 60,
  },
  tabLabel: {
    fontSize: 10,
    fontWeight: '500',
    color: '#6b7280',
  },
  tabLabelFocused: {
    color: '#4f7cff',
    fontWeight: '700',
  },
  icon: {
    fontSize: 16,
    fontWeight: '700',
  },
});
