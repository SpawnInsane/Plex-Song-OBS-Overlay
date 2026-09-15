/^[[:space:]]*(-[[:space:]]*)?uses[[:space:]]*:/ {
  reference = $0
  sub(/[[:space:]]+#.*$/, "", reference)
  sub(/^[[:space:]]*(-[[:space:]]*)?uses[[:space:]]*:[[:space:]]*/, "", reference)
  sub(/[[:space:]]+$/, "", reference)
  if (reference ~ /^\.\//) next
  revision = reference
  has_revision = sub(/^.*@/, "", revision)
  if (!has_revision || length(revision) != 40 || revision ~ /[^0-9a-f]/) {
    print FILENAME ":" FNR ": mutable action reference: " $0
    invalid = 1
  }
}
END { exit invalid }
