#!/usr/bin/env python3
"""
Skill: webapp-testing
Script: playwright_runner.py
Purpose: Run basic Playwright browser tests
Usage: python playwright_runner.py <url> [--screenshot]
Output: JSON with page info, health status, and optional screenshot path
Note: Playwright is supplied by the DevKit venv; Chromium installs lazily on first use.
Screenshots: Saved to system temp directory (auto-cleaned by OS)
"""
import sys
import json
import os
import subprocess
import tempfile
from datetime import datetime

# Fix Windows console encoding for Unicode output
try:
    sys.stdout.reconfigure(encoding='utf-8', errors='replace')
    sys.stderr.reconfigure(encoding='utf-8', errors='replace')
except AttributeError:
    pass  # Python < 3.7

try:
    from playwright.sync_api import sync_playwright
    PLAYWRIGHT_AVAILABLE = True
except ImportError:
    PLAYWRIGHT_AVAILABLE = False


def _install_chromium() -> tuple[bool, str]:
    """Download Chromium for the active Playwright installation on demand."""
    try:
        result = subprocess.run(
            [sys.executable, "-m", "playwright", "install", "chromium"],
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError as exc:
        return False, str(exc)
    if result.returncode == 0:
        return True, ""
    return False, (result.stderr or result.stdout or "unknown error").strip()


def _launch_chromium(playwright):
    """Launch Chromium, installing its browser binary once if it is absent."""
    try:
        return playwright.chromium.launch(headless=True), None
    except Exception as exc:
        message = str(exc)
        if "Executable doesn't exist" not in message and "executable doesn't exist" not in message:
            return None, message

    installed, detail = _install_chromium()
    if not installed:
        return None, (
            "Chromium is not installed and automatic setup failed: "
            f"{detail}. Run: {sys.executable} -m playwright install chromium"
        )
    try:
        return playwright.chromium.launch(headless=True), None
    except Exception as exc:
        return None, f"Chromium setup completed but browser launch failed: {exc}"


def _playwright_missing_result() -> dict:
    """Return a consistent remediation payload when the venv lacks Playwright."""
    return {
        "error": "Playwright not installed in the active Python environment",
        "fix": "Run leedevkit once to repair its venv, then retry this command.",
    }


def run_basic_test(url: str, take_screenshot: bool = False) -> dict:
    """Run basic browser test on URL."""
    if not PLAYWRIGHT_AVAILABLE:
        return _playwright_missing_result()
    
    result = {
        "url": url,
        "timestamp": datetime.now().isoformat(),
        "status": "pending"
    }
    
    try:
        with sync_playwright() as p:
            browser, launch_error = _launch_chromium(p)
            if launch_error:
                return {
                    "url": url,
                    "status": "error",
                    "error": launch_error,
                    "summary": f"[X] Error: {launch_error[:100]}",
                }
            context = browser.new_context(
                viewport={"width": 1280, "height": 720},
                user_agent="Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
            )
            page = context.new_page()
            
            # Navigate
            response = page.goto(url, wait_until="networkidle", timeout=30000)
            
            # Basic info
            result["page"] = {
                "title": page.title(),
                "url": page.url,
                "status_code": response.status if response else None
            }
            
            # Health checks
            result["health"] = {
                "loaded": response.ok if response else False,
                "has_title": bool(page.title()),
                "has_h1": page.locator("h1").count() > 0,
                "has_links": page.locator("a").count() > 0,
                "has_images": page.locator("img").count() > 0
            }
            
            # Console errors
            console_errors = []
            page.on("console", lambda msg: console_errors.append(msg.text) if msg.type == "error" else None)
            
            # Performance metrics
            result["performance"] = {
                "dom_content_loaded": page.evaluate("window.performance.timing.domContentLoadedEventEnd - window.performance.timing.navigationStart"),
                "load_complete": page.evaluate("window.performance.timing.loadEventEnd - window.performance.timing.navigationStart")
            }
            
            # Screenshot - uses system temp directory (cross-platform, auto-cleaned)
            if take_screenshot:
                # Cross-platform: Windows=%TEMP%, Linux/macOS=/tmp
                screenshot_dir = os.path.join(tempfile.gettempdir(), "maestro_screenshots")
                os.makedirs(screenshot_dir, exist_ok=True)
                screenshot_path = os.path.join(screenshot_dir, f"screenshot_{datetime.now().strftime('%Y%m%d_%H%M%S')}.png")
                page.screenshot(path=screenshot_path, full_page=True)
                result["screenshot"] = screenshot_path
                result["screenshot_note"] = "Saved to temp directory (auto-cleaned by OS)"
            
            # Element counts
            result["elements"] = {
                "links": page.locator("a").count(),
                "buttons": page.locator("button").count(),
                "inputs": page.locator("input").count(),
                "images": page.locator("img").count(),
                "forms": page.locator("form").count()
            }
            
            browser.close()
            
            result["status"] = "success" if result["health"]["loaded"] else "failed"
            result["summary"] = "[OK] Page loaded successfully" if result["status"] == "success" else "[X] Page failed to load"
            
    except Exception as e:
        result["status"] = "error"
        result["error"] = str(e)
        result["summary"] = f"[X] Error: {str(e)[:100]}"
    
    return result


def run_accessibility_check(url: str) -> dict:
    """Run basic accessibility check."""
    if not PLAYWRIGHT_AVAILABLE:
        return _playwright_missing_result()
    
    result = {"url": url, "accessibility": {}}
    
    try:
        with sync_playwright() as p:
            browser, launch_error = _launch_chromium(p)
            if launch_error:
                return {"url": url, "status": "error", "error": launch_error}
            page = browser.new_page()
            page.goto(url, wait_until="networkidle", timeout=30000)
            
            # Basic a11y checks
            result["accessibility"] = {
                "images_with_alt": page.locator("img[alt]").count(),
                "images_without_alt": page.locator("img:not([alt])").count(),
                "buttons_with_label": page.locator("button[aria-label], button:has-text('')").count(),
                "links_with_text": page.locator("a:has-text('')").count(),
                "form_labels": page.locator("label").count(),
                "headings": {
                    "h1": page.locator("h1").count(),
                    "h2": page.locator("h2").count(),
                    "h3": page.locator("h3").count()
                }
            }
            
            browser.close()
            result["status"] = "success"
            
    except Exception as e:
        result["status"] = "error"
        result["error"] = str(e)
    
    return result


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(json.dumps({
            "error": "Usage: python playwright_runner.py <url> [--screenshot] [--a11y]",
            "examples": [
                "python playwright_runner.py https://example.com",
                "python playwright_runner.py https://example.com --screenshot",
                "python playwright_runner.py https://example.com --a11y"
            ]
        }, indent=2))
        sys.exit(1)
    
    url = sys.argv[1]
    take_screenshot = "--screenshot" in sys.argv
    check_a11y = "--a11y" in sys.argv
    
    if check_a11y:
        result = run_accessibility_check(url)
    else:
        result = run_basic_test(url, take_screenshot)
    
    print(json.dumps(result, indent=2))
