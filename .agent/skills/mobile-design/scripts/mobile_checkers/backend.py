"""Mobile Backend checks: secure storage, offline handling, push notifications."""

import re

from ._base import MobileAuditContext


class MobileBackendChecker:
    """Backend integration checks for mobile apps."""

    def run(self, ctx: MobileAuditContext) -> list[dict[str, str]]:
        findings: list[dict[str, str]] = []

        # 8.1 Secure Storage
        has_async_storage = bool(re.search(
            r'AsyncStorage|@react-native-async-storage', ctx.content,
        ))
        has_secure_storage = bool(re.search(
            r'SecureStore|Keychain|EncryptedSharedPreferences', ctx.content,
        ))
        has_token_storage = bool(re.search(
            r'token|jwt|auth.*storage', ctx.content, re.IGNORECASE,
        ))
        if has_token_storage and has_async_storage and not has_secure_storage:
            findings.append({
                "severity": "issue",
                "message": (
                    f"[Security] {ctx.filename}: Storing auth tokens in AsyncStorage "
                    "(insecure). Use SecureStore (iOS) / EncryptedSharedPreferences "
                    "(Android)."
                ),
            })

        # 8.2 Offline Handling
        has_network = bool(re.search(
            r'fetch|axios|netinfo|@react-native-community/netinfo', ctx.content,
        ))
        has_offline = bool(re.search(
            r'offline|isConnected|netInfo|cache.*offline', ctx.content,
        ))
        if has_network and not has_offline:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Offline] {ctx.filename}: Network requests detected without "
                    "offline handling. Consider NetInfo for connection status."
                ),
            })

        # 8.3 Push Notifications
        has_push = bool(re.search(
            r'Notifications|pushNotification|Firebase\.messaging|PushNotificationIOS',
            ctx.content,
        ))
        has_push_handler = bool(re.search(
            r'onNotification|addNotificationListener|notification\.open', ctx.content,
        ))
        if has_push and not has_push_handler:
            findings.append({
                "severity": "warning",
                "message": (
                    f"[Push] {ctx.filename}: Push notifications imported but no "
                    "handler found. May miss notifications."
                ),
            })

        return findings
