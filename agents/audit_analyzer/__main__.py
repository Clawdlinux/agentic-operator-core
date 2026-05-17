"""CLI entrypoint for the audit analyzer.

Two modes:
  --once    Run a single analyzer pass and exit. Used as a Kubernetes CronJob.
  --serve   Run the FastAPI surface + analyzer-on-demand. Used as a long-lived
            Deployment when commercial tier needs an interactive REST API.
"""

from __future__ import annotations

import argparse
import json
import logging
import sys

from .api import create_app
from .config import AuditAnalyzerConfig
from .pipeline import AnalyzerRunner


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="audit-analyzer")
    sub = parser.add_subparsers(dest="cmd", required=True)
    sub.add_parser("once", help="Run a single analyzer pass and exit")
    serve = sub.add_parser("serve", help="Run the FastAPI surface")
    serve.add_argument("--host", default="0.0.0.0")
    serve.add_argument("--port", type=int, default=9595)
    args = parser.parse_args(argv)

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(name)s %(levelname)s %(message)s",
    )
    cfg = AuditAnalyzerConfig()

    if args.cmd == "once":
        runner = AnalyzerRunner(cfg)
        cards = runner.run()
        out = [c.to_dict() for c in cards]
        json.dump({"cards": out, "count": len(out)}, sys.stdout, default=str)
        sys.stdout.write("\n")
        return 0

    if args.cmd == "serve":
        try:
            import uvicorn  # type: ignore[import-not-found]
        except ImportError:
            sys.stderr.write("audit-analyzer serve: install uvicorn[standard]\n")
            return 2
        app = create_app(cfg)
        uvicorn.run(app, host=args.host, port=args.port, log_level="info")
        return 0

    return 2


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
