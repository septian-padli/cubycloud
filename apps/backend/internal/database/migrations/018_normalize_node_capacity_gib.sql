UPDATE storage_nodes
SET capacity_bytes = (capacity_bytes / 1000000000) * 1073741824
WHERE capacity_bytes > 0
  AND capacity_bytes % 1000000000 = 0
  AND capacity_bytes % 1073741824 != 0;
