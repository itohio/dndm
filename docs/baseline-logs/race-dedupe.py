#!/usr/bin/env python3
"""Group Go race-detector reports by the first in-repo frame of each access.

Usage: python3 race-dedupe.py <go-test-v-output> <repo-path-prefix>
Prints one line per unique access pair with the tests that hit it.
Stdlib only; reads the log, never runs code.
"""
import collections
import re
import sys


def first_repo_frame(section, start, prefix):
    for k in range(start, len(section)):
        line = section[k].strip()
        if not line:
            break
        if prefix in line and "_testmain.go" not in line:
            return section[k - 1].strip().split("(0x")[0] + " " + line.split(" ")[0].replace(prefix, "")
    return "<no repo frame>"


def main(path, prefix):
    lines = open(path, encoding="utf-8").read().split("\n")
    pairs = collections.OrderedDict()
    test, i, total = None, 0, 0
    while i < len(lines):
        m = re.match(r"=== RUN\s+(\S+)", lines[i])
        if m:
            test = m.group(1)
        if lines[i].startswith("WARNING: DATA RACE"):
            total += 1
            j = i + 1
            while j < len(lines) and not lines[j].startswith("=================="):
                j += 1
            section = lines[i + 1 : j]
            accesses = [
                first_repo_frame(section, k + 1, prefix)
                for k, line in enumerate(section)
                if re.match(r"(Read|Write|Previous read|Previous write) at \S+ by", line)
            ][:2]
            pairs.setdefault(" <-> ".join(sorted(accesses)), set()).add(test or "<no -v test name>")
            i = j
        i += 1
    print(f"{total} reports, {len(pairs)} unique access pairs")
    for key, tests in pairs.items():
        print(f"{key} | {','.join(sorted(tests))}")


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
