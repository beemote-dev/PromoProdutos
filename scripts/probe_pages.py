"""Live diagnostics only: no challenge interaction, history updates or alerts."""
import argparse
import datetime
import json
import os
from pathlib import Path

from playwright.sync_api import sync_playwright

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--output", default="work/probes")
parser.add_argument("--chromium", default="/usr/bin/chromium")
args = parser.parse_args()
output = Path(args.output)
output.mkdir(parents=True, exist_ok=True)
proxy = {"server": os.environ["HTTPS_PROXY"]} if os.environ.get("HTTPS_PROXY") else None

with sync_playwright() as playwright:
    browser = playwright.chromium.launch(
        executable_path=args.chromium,
        headless=True,
        proxy=proxy,
        args=["--no-sandbox"],
    )
    products = json.loads(Path("config/products.json").read_text())
    for product in products:
        page = browser.new_page(locale="pt-BR")
        record = {
            "id": product["id"],
            "at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "requested_url": product["url"],
            "method": "playwright",
        }
        try:
            response = page.goto(
                product["url"], wait_until="domcontentloaded", timeout=45000
            )
            page.wait_for_timeout(5000)
            record.update(
                status=response.status if response else None,
                final_url=page.url,
                title=page.title(),
                text=page.locator("body").inner_text()[:20000],
            )
            (output / f"{product['id']}-browser.html").write_text(page.content())
            page.screenshot(path=str(output / f"{product['id']}.png"), full_page=False)
        except Exception as error:
            record["error"] = str(error)
        finally:
            page.close()
        (output / f"{product['id']}-browser.json").write_text(
            json.dumps(record, ensure_ascii=False, indent=2) + "\n"
        )
        print(
            json.dumps({k: v for k, v in record.items() if k != "text"}, ensure_ascii=False),
            flush=True,
        )
    browser.close()
