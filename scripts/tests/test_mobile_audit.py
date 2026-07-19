"""Tests for Mobile audit checkers — integration and unit tests."""

import os
import sys

import pytest

# Allow importing from the mobile audit scripts directory
_MOBILE_DIR = os.path.join(
    os.path.dirname(__file__),
    "..",
    "..",
    ".agent",
    "skills",
    "mobile-design",
    "scripts",
)
sys.path.insert(0, _MOBILE_DIR)


# ---------------------------------------------------------------------------
# Test fixtures
# ---------------------------------------------------------------------------


@pytest.fixture
def react_native_content():
    return """import React from 'react';
import { View, Text, FlatList, Pressable, SafeAreaView } from 'react-native';
import { useCallback } from 'react';

function Item({ item }) { return <Text>{item.name}</Text>; }
const MemoItem = React.memo(Item);

export default function App() {
  const data = [{id: '1', name: 'A'}];
  const renderItem = useCallback(({item}) => <MemoItem item={item} />, []);
  return (
    <SafeAreaView>
      <FlatList
        data={data}
        renderItem={renderItem}
        keyExtractor={(item) => item.id}
      />
      <Pressable accessibilityLabel="Submit"
        style={({pressed}) => ({opacity: pressed ? 0.5 : 1})}>
        <Text>Submit</Text>
      </Pressable>
    </SafeAreaView>
  );
}"""


@pytest.fixture
def bad_react_native():
    return """import React from 'react';
import { View, Text, ScrollView, Pressable } from 'react-native';

export default function App() {
  const items = [1, 2, 3, 4, 5, 6, 7, 8];
  return (
    <View style={{height: 20}}>
      <ScrollView>
        {items.map(i => <Text key={i}>{i}</Text>)}
      </ScrollView>
      <Pressable onPress={() => {}}><Text>Click</Text></Pressable>
      <Text style={{fontSize: 10, color: '#000000'}}>Tiny</Text>
    </View>
  );
}"""


# ---------------------------------------------------------------------------
# Integration tests
# ---------------------------------------------------------------------------


class TestMobileAuditorIntegration:
    def test_compliant_rn_passes(self, tmp_path, react_native_content):
        f = tmp_path / "App.tsx"
        f.write_text(react_native_content, encoding="utf-8")

        from mobile_audit import MobileAuditor

        auditor = MobileAuditor()
        auditor.audit_file(str(f))
        report = auditor.get_report()

        assert report["files_checked"] == 1
        assert len(report["issues"]) == 0, f"Issues: {report['issues']}"

    def test_bad_rn_has_issues(self, tmp_path, bad_react_native):
        f = tmp_path / "BadApp.tsx"
        f.write_text(bad_react_native, encoding="utf-8")

        from mobile_audit import MobileAuditor

        auditor = MobileAuditor()
        auditor.audit_file(str(f))
        report = auditor.get_report()

        assert report["files_checked"] == 1
        assert len(report["issues"]) >= 1
        assert len(report["warnings"]) >= 1

    def test_non_mobile_skipped(self, tmp_path):
        f = tmp_path / "server.py"
        f.write_text("print('hello')\n", encoding="utf-8")

        from mobile_audit import MobileAuditor

        auditor = MobileAuditor()
        auditor.audit_file(str(f))

        assert auditor.files_checked == 0  # skipped


# ---------------------------------------------------------------------------
# Touch psychology checker unit tests
# ---------------------------------------------------------------------------


class TestTouchPsychologyChecker:
    @staticmethod
    def _ctx(content="", filename="test.tsx", is_react_native=True):
        from mobile_checkers._base import MobileAuditContext

        return MobileAuditContext(
            filename=filename,
            content=content,
            is_react_native=is_react_native,
        )

    def test_touch_target_too_small(self):
        from mobile_checkers.touch_psychology import TouchPsychologyChecker

        ctx = self._ctx(content="<View style={{width: 20}}>x</View>")
        findings = TouchPsychologyChecker().run(ctx)
        assert any("Touch target size" in f["message"] for f in findings)

    def test_touch_target_ok(self):
        from mobile_checkers.touch_psychology import TouchPsychologyChecker

        ctx = self._ctx(content="<View style={{width: 50}}>x</View>")
        findings = TouchPsychologyChecker().run(ctx)
        assert not any("Touch target size" in f["message"] for f in findings)

    def test_no_haptics(self):
        from mobile_checkers.touch_psychology import TouchPsychologyChecker

        ctx = self._ctx(content="<Pressable onPress={submit}>Go</Pressable>")
        findings = TouchPsychologyChecker().run(ctx)
        assert any("haptic" in f["message"] for f in findings)

    def test_swipe_without_visible_buttons(self):
        from mobile_checkers.touch_psychology import TouchPsychologyChecker

        ctx = self._ctx(
            content="<Swipeable onSwipe={handler}><Text>Swipe</Text></Swipeable>"
        )
        findings = TouchPsychologyChecker().run(ctx)
        assert any("Swipe gestures" in f["message"] for f in findings)


# ---------------------------------------------------------------------------
# Performance checker unit tests
# ---------------------------------------------------------------------------


class TestMobilePerformanceChecker:
    @staticmethod
    def _ctx(content="", filename="test.tsx", is_react_native=True):
        from mobile_checkers._base import MobileAuditContext

        return MobileAuditContext(
            filename=filename,
            content=content,
            is_react_native=is_react_native,
        )

    def test_scrollview_with_map_critical(self):
        from mobile_checkers.performance import MobilePerformanceChecker

        ctx = self._ctx(
            content="<ScrollView>{items.map(i => <Text>{i}</Text>)}</ScrollView>"
        )
        findings = MobilePerformanceChecker().run(ctx)
        assert any("ScrollView with .map()" in f["message"] for f in findings)

    def test_flatlist_without_keyextractor(self):
        from mobile_checkers.performance import MobilePerformanceChecker

        ctx = self._ctx(content="<FlatList data={data} renderItem={render} />")
        findings = MobilePerformanceChecker().run(ctx)
        assert any("keyExtractor" in f["message"] for f in findings)

    def test_index_as_key(self):
        from mobile_checkers.performance import MobilePerformanceChecker

        ctx = self._ctx(content="<Text key={index}>item</Text>")
        findings = MobilePerformanceChecker().run(ctx)
        assert any("index as key" in f["message"].lower() for f in findings)

    def test_memory_leak_no_cleanup(self):
        from mobile_checkers.performance import MobilePerformanceChecker

        ctx = self._ctx(
            content="""
useEffect(() => {
  const sub = api.subscribe();
}, []);
"""
        )
        findings = MobilePerformanceChecker().run(ctx)
        assert any("Memory Leak" in f["message"] for f in findings)

    def test_use_native_driver_false(self):
        from mobile_checkers.performance import MobilePerformanceChecker

        ctx = self._ctx(
            content="""
Animated.timing(val, {useNativeDriver: false}).start();
"""
        )
        findings = MobilePerformanceChecker().run(ctx)
        assert any("useNativeDriver: false" in f["message"] for f in findings)

    def test_many_console_logs(self):
        from mobile_checkers.performance import MobilePerformanceChecker

        content = "\n".join([f'console.log("msg{i}");' for i in range(10)])
        ctx = self._ctx(content=content)
        findings = MobilePerformanceChecker().run(ctx)
        assert any("console.log" in f["message"] for f in findings)


# ---------------------------------------------------------------------------
# Color checker unit tests
# ---------------------------------------------------------------------------


class TestMobileColorChecker:
    @staticmethod
    def _ctx(content="", filename="test.tsx"):
        from mobile_checkers._base import MobileAuditContext

        return MobileAuditContext(
            filename=filename, content=content, is_react_native=True
        )

    def test_pure_black_detected(self):
        from mobile_checkers.color import MobileColorChecker

        ctx = self._ctx(content='<Text style={{color: "#000000"}}>Black</Text>')
        findings = MobileColorChecker().run(ctx)
        assert any("Pure black" in f["message"] for f in findings)

    def test_no_dark_mode(self):
        from mobile_checkers.color import MobileColorChecker

        ctx = self._ctx(content="<View><Text>Hello</Text></View>")
        findings = MobileColorChecker().run(ctx)
        assert any("No dark mode" in f["message"] for f in findings)

    def test_has_dark_mode_ok(self):
        from mobile_checkers.color import MobileColorChecker

        ctx = self._ctx(content='import { useColorScheme } from "react-native";')
        findings = MobileColorChecker().run(ctx)
        assert not any("No dark mode" in f["message"] for f in findings)


# ---------------------------------------------------------------------------
# Platform checks
# ---------------------------------------------------------------------------


class TestPlatformIOSChecker:
    def test_no_safe_area(self):
        from mobile_checkers._base import MobileAuditContext
        from mobile_checkers.platform_ios import PlatformIOSChecker

        ctx = MobileAuditContext(
            filename="test.tsx",
            content="<View><Text>Hi</Text></View>",
            is_react_native=True,
        )
        findings = PlatformIOSChecker().run(ctx)
        assert any("SafeArea" in f["message"] for f in findings)

    def test_has_safe_area_ok(self):
        from mobile_checkers._base import MobileAuditContext
        from mobile_checkers.platform_ios import PlatformIOSChecker

        ctx = MobileAuditContext(
            filename="test.tsx",
            content="<SafeAreaView><Text>Hi</Text></SafeAreaView>",
            is_react_native=True,
        )
        findings = PlatformIOSChecker().run(ctx)
        assert not any("SafeArea" in f["message"] for f in findings)


class TestPlatformAndroidChecker:
    def test_no_ripple(self):
        from mobile_checkers._base import MobileAuditContext
        from mobile_checkers.platform_android import PlatformAndroidChecker

        ctx = MobileAuditContext(
            filename="test.tsx",
            content="<Pressable onPress={() => {}}><Text>Go</Text></Pressable>",
            is_react_native=True,
        )
        findings = PlatformAndroidChecker().run(ctx)
        assert any("ripple" in f["message"] for f in findings)

    def test_no_material3_dynamic_color(self):
        from mobile_checkers._base import MobileAuditContext
        from mobile_checkers.platform_android import PlatformAndroidChecker

        ctx = MobileAuditContext(
            filename="test.tsx",
            content="<View><Text>Hi</Text></View>",
            is_react_native=True,
        )
        findings = PlatformAndroidChecker().run(ctx)
        assert any("Material 3 dynamic color" in f["message"] for f in findings)


# ---------------------------------------------------------------------------
# Backend checker unit tests
# ---------------------------------------------------------------------------


class TestMobileBackendChecker:
    def test_async_storage_for_tokens(self):
        from mobile_checkers._base import MobileAuditContext
        from mobile_checkers.backend import MobileBackendChecker

        ctx = MobileAuditContext(
            filename="test.tsx",
            content="""
import AsyncStorage from '@react-native-async-storage/async-storage';
const token = await AsyncStorage.getItem('jwt');
""",
            is_react_native=True,
        )
        findings = MobileBackendChecker().run(ctx)
        assert any("AsyncStorage" in f["message"] for f in findings)

    def test_network_without_offline(self):
        from mobile_checkers._base import MobileAuditContext
        from mobile_checkers.backend import MobileBackendChecker

        ctx = MobileAuditContext(
            filename="test.tsx",
            content="fetch('https://api.example.com/data')",
        )
        findings = MobileBackendChecker().run(ctx)
        assert any("offline handling" in f["message"] for f in findings)
