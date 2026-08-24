---
name: mobile-design
description: Mobile-first design thinking and decision-making for iOS and Android apps. Touch interaction, performance patterns, platform conventions. Teaches principles, not fixed values. Use when building React Native, Flutter, or native mobile apps.
allowed-tools: Read, Glob, Grep, Bash
---

# Mobile Design System

> **Philosophy:** Touch-first. Battery-conscious. Platform-respectful. Offline-capable.
> **Core Principle:** Mobile is NOT a small desktop. THINK mobile constraints, ASK platform choice.

---

## 🔧 Runtime Scripts

**Execute these for validation (don't read, just run):**

| Script | Purpose | Usage |
|--------|---------|-------|
| `.agent/skills/mobile-design/scripts/mobile_audit.py` | Mobile UX & Touch Audit | `.leedevkit/.venv/bin/python3 .agent/skills/mobile-design/scripts/mobile_audit.py <project_path>` |

---

## 📚 Reading Map

Read only the files relevant to the platform and task. **Start with** `mobile-design-thinking.md`.

### Universal

| File | When to Read |
|------|--------------|
| [mobile-design-thinking.md](mobile-design-thinking.md) | First — prevents memorized patterns, forces context-based thinking |
| [touch-psychology.md](touch-psychology.md) | Touch targets, gestures, haptics, thumb zone |
| [mobile-performance.md](mobile-performance.md) | RN/Flutter performance, 60fps, memory |
| [mobile-backend.md](mobile-backend.md) | Push notifications, offline sync, mobile API |
| [mobile-testing.md](mobile-testing.md) | Testing pyramid, E2E, platform-specific tests |
| [mobile-debugging.md](mobile-debugging.md) | Native vs JS debugging, Flipper, Logcat |

### Platform-Specific

| Platform | File |
|----------|------|
| iOS | [platform-ios.md](platform-ios.md) |
| Android | [platform-android.md](platform-android.md) |
| Cross-platform | Both above |

---

## Ask Before Assuming

Ask only when the choice materially affects architecture or UX:

| Aspect | Ask If Missing |
|--------|----------------|
| Platform | iOS, Android, or both? |
| Framework | React Native, Flutter, or native? |
| Offline | What must work without network? |

### AI Default Tendencies to Avoid

| Default Tendency | Prefer Instead |
|------------------|----------------|
| `ScrollView` for lists | `FlatList` / `FlashList` / `ListView.builder` |
| Inline `renderItem` | `useCallback` + `React.memo` |
| `AsyncStorage` for tokens | `SecureStore` / `Keychain` |
| Same stack for all projects | What does THIS project need? |
| Skipping platform checks | iOS feels iOS, Android feels Android |

---

## 🚫 Mobile Anti-Patterns

### Performance

| ❌ Never | ✅ Always |
|----------|-----------|
| `ScrollView` for long lists | `FlatList` / `FlashList` / `ListView.builder` |
| Inline `renderItem` function | `useCallback` + `React.memo` |
| Missing `keyExtractor` | Stable unique ID from data |
| `useNativeDriver: false` | `useNativeDriver: true` |
| `console.log` in production | Remove before release |

### Touch / UX

| ❌ Never | ✅ Always |
|----------|-----------|
| Touch target < 44px | Minimum 44pt (iOS) / 48dp (Android) |
| Spacing < 8px | Minimum 8-12px gap |
| Gesture-only interactions | Provide visible button alternative |
| No loading state | ALWAYS show loading feedback |
| No error state | Show error with retry option |
| No offline handling | Graceful degradation, cached data |

### Security

| ❌ Never | ✅ Always |
|----------|-----------|
| Token in `AsyncStorage` | `SecureStore` / `Keychain` / `EncryptedSharedPreferences` |
| Hardcode API keys | Environment variables, secure storage |
| Skip SSL pinning | Pin certificates in production |
| Log sensitive data | Never log tokens, passwords, PII |

### Architecture

| ❌ Never | ✅ Always |
|----------|-----------|
| Business logic in UI | Service layer separation |
| Global state for everything | Local state default, lift when needed |
| Deep linking as afterthought | Plan deep links from day one |
| Skip dispose/cleanup | Clean up subscriptions, timers |

---

## 📱 Platform Decision Matrix

### When to Unify vs Diverge

```
                    UNIFY (same on both)          DIVERGE (platform-specific)
                    ───────────────────           ──────────────────────────
Business Logic      ✅ Always                     -
Data Layer          ✅ Always                     -
Core Features       ✅ Always                     -

Navigation          -                             ✅ iOS: edge swipe, Android: back button
Gestures            -                             ✅ Platform-native feel
Icons               -                             ✅ SF Symbols vs Material Icons
Date Pickers        -                             ✅ Native pickers feel right
Modals/Sheets       -                             ✅ iOS: bottom sheet vs Android: dialog
Typography          -                             ✅ SF Pro vs Roboto (or custom)
Error Dialogs       -                             ✅ Platform conventions for alerts
```

### Quick Reference: Platform Defaults

| Element | iOS | Android |
|---------|-----|---------|
| **Primary Font** | SF Pro / SF Compact | Roboto |
| **Min Touch Target** | 44pt × 44pt | 48dp × 48dp |
| **Back Navigation** | Edge swipe left | System back button/gesture |
| **Bottom Tab Icons** | SF Symbols | Material Symbols |
| **Action Sheet** | UIActionSheet from bottom | Bottom Sheet / Dialog |
| **Progress** | Spinner | Linear progress (Material) |
| **Pull to Refresh** | Native UIRefreshControl | SwipeRefreshLayout |

---

## 🧠 Mobile UX Psychology (Quick Reference)

### Fitts' Law for Touch

```
Desktop: Cursor is precise (1px)
Mobile:  Finger is imprecise (~7mm contact area)

→ Touch targets MUST be 44-48px minimum
→ Important actions in THUMB ZONE (bottom of screen)
→ Destructive actions AWAY from easy reach
```

### Thumb Zone (One-Handed Usage)

```
┌─────────────────────────────┐
│      HARD TO REACH          │ ← Navigation, menu, back
│        (stretch)            │
├─────────────────────────────┤
│      OK TO REACH            │ ← Secondary actions
│       (natural)             │
├─────────────────────────────┤
│      EASY TO REACH          │ ← PRIMARY CTAs, tab bar
│    (thumb's natural arc)    │ ← Main content interaction
└─────────────────────────────┘
        [  HOME  ]
```

For deep dive: [touch-psychology.md](touch-psychology.md)

---

## ⚡ Performance Principles (Quick Reference)

### React Native Critical Rules

```typescript
// ✅ CORRECT: Memoized renderItem + React.memo wrapper
const ListItem = React.memo(({ item }: { item: Item }) => (
  <View style={styles.item}>
    <Text>{item.title}</Text>
  </View>
));

const renderItem = useCallback(
  ({ item }: { item: Item }) => <ListItem item={item} />,
  []
);

// ✅ CORRECT: FlatList with all optimizations
<FlatList
  data={items}
  renderItem={renderItem}
  keyExtractor={(item) => item.id}  // Stable ID, NOT index
  getItemLayout={(data, index) => ({
    length: ITEM_HEIGHT,
    offset: ITEM_HEIGHT * index,
    index,
  })}
  removeClippedSubviews={true}
  maxToRenderPerBatch={10}
  windowSize={5}
/>
```

### Flutter Critical Rules

```dart
// ✅ CORRECT: const constructors prevent rebuilds
class MyWidget extends StatelessWidget {
  const MyWidget({super.key}); // CONST!

  @override
  Widget build(BuildContext context) {
    return const Column( // CONST!
      children: [
        Text('Static content'),
        MyConstantWidget(),
      ],
    );
  }
}

// ✅ CORRECT: Targeted state with ValueListenableBuilder
ValueListenableBuilder<int>(
  valueListenable: counter,
  builder: (context, value, child) => Text('$value'),
  child: const ExpensiveWidget(), // Won't rebuild!
)
```

### Animation Performance

```
GPU-accelerated (FAST):     CPU-bound (SLOW):
├── transform               ├── width, height
├── opacity                 ├── top, left, right, bottom
└── (use these ONLY)        ├── margin, padding
                            └── (AVOID animating these)
```

For complete guide: [mobile-performance.md](mobile-performance.md)

---

## 🔧 Framework Decision Tree

```
WHAT ARE YOU BUILDING?
        │
        ├── Need OTA updates + rapid iteration + web team
        │   └── ✅ React Native + Expo
        │
        ├── Need pixel-perfect custom UI + performance critical
        │   └── ✅ Flutter
        │
        ├── Deep native features + single platform focus
        │   ├── iOS only → SwiftUI
        │   └── Android only → Kotlin + Jetpack Compose
        │
        ├── Existing RN codebase + new features
        │   └── ✅ React Native (bare workflow)
        │
        └── Enterprise + existing Flutter codebase
            └── ✅ Flutter
```

For complete decision trees: [decision-trees.md](decision-trees.md)

---

## 📋 Pre-Development Checklist

### Before Starting ANY Mobile Project

- [ ] **Platform confirmed?** (iOS / Android / Both)
- [ ] **Framework chosen?** (RN / Flutter / Native)
- [ ] **Navigation pattern decided?** (Tabs / Stack / Drawer)
- [ ] **State management selected?** (Zustand / Redux / Riverpod / BLoC)
- [ ] **Offline requirements known?**
- [ ] **Deep linking planned from day one?**
- [ ] **Target devices defined?** (Phone / Tablet / Both)

### Before Every Screen

- [ ] **Touch targets ≥ 44-48px?**
- [ ] **Primary CTA in thumb zone?**
- [ ] **Loading state exists?**
- [ ] **Error state with retry exists?**
- [ ] **Offline handling considered?**
- [ ] **Platform conventions followed?**

### Before Release

- [ ] **console.log removed?**
- [ ] **SecureStore for sensitive data?**
- [ ] **SSL pinning enabled?**
- [ ] **Lists optimized (memo, keyExtractor)?**
- [ ] **Memory cleanup on unmount?**
- [ ] **Tested on low-end devices?**
- [ ] **Accessibility labels on all interactive elements?**

---

## 📚 Reference Files

| File | When to Use |
|------|-------------|
| [mobile-design-thinking.md](mobile-design-thinking.md) | **FIRST!** Anti-memorization, forces context-based thinking |
| [touch-psychology.md](touch-psychology.md) | Touch interaction, Fitts' Law, gesture design |
| [mobile-performance.md](mobile-performance.md) | Optimizing RN/Flutter, 60fps, memory/battery |
| [platform-ios.md](platform-ios.md) | iOS-specific design, HIG compliance |
| [platform-android.md](platform-android.md) | Android-specific design, Material Design 3 |
| [mobile-navigation.md](mobile-navigation.md) | Navigation patterns, deep linking |
| [mobile-typography.md](mobile-typography.md) | Type scale, system fonts, accessibility |
| [mobile-color-system.md](mobile-color-system.md) | OLED optimization, dark mode, battery |
| [decision-trees.md](decision-trees.md) | Framework, state, storage decisions |

---

> **Remember:** Mobile users are impatient, interrupted, and using imprecise fingers on small screens. Design for the WORST conditions: bad network, one hand, bright sun, low battery. If it works there, it works everywhere.
