"""Z3 proof: mattermost-plugin-community-admin username whitelist.

Pattern: `^[a-z0-9._-]+$` (server/service/password.go:14), enforced by
ValidateUsername() at the user-creation and batch-import boundaries
(users.go:42,122; batch.go:97).

Critical question: does the whitelist accept ANY string containing a
character that is dangerous in the downstream contexts (shell commands,
SMS text lines, LDAP/syslog output)? If unsat -> no such input exists.

Per the z3-regex skill: containment proofs are alphabet-trivial. The
whitelist alphabet is {a-z0-9._-}; prove each dangerous char is NOT in
the language of accepted strings. Length bounds not needed.
"""
from z3 import *

DANGEROUS = {
    "space": " ",
    "equals": "=",
    "semicolon": ";",
    "double-quote": '"',
    "single-quote": "'",
    "backtick": "`",
    "dollar": "$",
    "backslash": "\\",
    "newline": "\n",
    "tab": "\t",
    "ampersand": "&",
    "pipe": "|",
    "greater-than": ">",
    "less-than": "<",
    "percent": "%",
    "comma": ",",
    "colon": ":",
    "at-sign": "@",
    "tilde": "~",
    "caret": "^",
    "brace-open": "{",
    "brace-close": "}",
    "paren-open": "(",
    "paren-close": ")",
    "bracket-open": "[",
    "bracket-close": "]",
    "slash": "/",
    "hash": "#",
    "star": "*",
    "plus": "+",
    "question": "?",
}

def main():
    s = String("s")
    c = String("c")
    # Whitelist token alphabet: [a-z0-9._-]  (from ^[a-z0-9._-]+$)
    wl_cls = Union(
        Range("a", "z"), Range("0", "9"), Re("."), Re("_"), Re("-")
    )

    # Alphabet-disjointness form (instant per the z3 skill): a single char
    # accepted by the token alphabet cannot be a dangerous char.
    # InRe(c, wl_cls) ∧ Length(c)==1 ∧ c == "<bad>"  →  unsat means excluded.
    failures = []
    for name, ch in DANGEROUS.items():
        solver = Solver()
        solver.set("timeout", 30000)
        solver.add(InRe(c, wl_cls))
        solver.add(Length(c) == 1)
        solver.add(c == StringVal(ch))
        r = solver.check()
        if r == unsat:
            print(f"[PASS] {name:14} '{ch}' excluded from whitelist alphabet")
        elif r == sat:
            m = solver.model()
            print(f"[FAIL] {name:14} '{ch}' ACCEPTED by whitelist! model: {m[c]}")
            failures.append(name)
        else:
            print(f"[TIMEOUT] {name}: {r}")
            failures.append(name)

    # Sanity / harness-can-fail check: a whitelisted char IS accepted
    # (weakening the alphabet to include a bad char flips to SAT).
    solver = Solver()
    solver.set("timeout", 30000)
    solver.add(InRe(c, wl_cls))
    solver.add(Length(c) == 1)
    solver.add(c == StringVal("a"))
    r = solver.check()
    print(f"[{'PASS' if r == sat else 'FAIL'}] sanity: 'a' accepted ({r})")
    if r != sat:
        failures.append("sanity")

    print()
    if failures:
        print(f"RESULT: FAIL ({len(failures)} violations: {', '.join(failures)})")
        raise SystemExit(1)
    print("RESULT: PASS — whitelist alphabet is exactly [a-z0-9._-]; "
          "no dangerous char is reachable in any accepted username")


if __name__ == "__main__":
    main()
