#!/usr/bin/env python3
"""Clear stuck SVN 1.14 work queue on SMB working copy (post-commit bump failure)."""
import sqlite3
import sys
from pathlib import Path

WC = Path(r"Y:\holyjing\starcrystalsvr")
DB = WC / ".svn" / "wc.db"


def main() -> int:
    if not DB.is_file():
        print(f"missing {DB}")
        return 1
    conn = sqlite3.connect(str(DB))
    cur = conn.cursor()
    cur.execute("SELECT COUNT(*) FROM WORK_QUEUE")
    n = cur.fetchone()[0]
    print(f"WORK_QUEUE items: {n}")
    if "--fix" not in sys.argv:
        cur.execute("SELECT id, work FROM WORK_QUEUE LIMIT 20")
        for row in cur.fetchall():
            print(" ", row)
        print("pass --fix to clear WORK_QUEUE and WC_LOCK")
        return 0
    cur.execute("DELETE FROM WORK_QUEUE")
    cur.execute("DELETE FROM WC_LOCK")
    conn.commit()
    conn.close()
    print("cleared; run: svn cleanup && svn update")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
