import re

with open('cmd/main.go', 'r') as f:
    content = f.read()

# 1. Add sourceCodeMap
if 'sourceCodeMap sync.Map' not in content:
    content = content.replace(
        'var (\n\tdb           *store.Store\n\tredisClient  *redis.Client\n\ttelemetryHub *ws.Hub\n)',
        'var (\n\tdb            *store.Store\n\tredisClient   *redis.Client\n\ttelemetryHub  *ws.Hub\n\tsourceCodeMap sync.Map\n)'
    )

# 2. Add or update runFinalization
with open('patch.go', 'r') as pf:
    patch_content = pf.read()

# Extract just the function from patch.go
func_match = re.search(r'func runFinalization\(.*', patch_content, re.DOTALL)
if func_match:
    func_code = func_match.group(0).strip()
    start_idx = content.find('func runFinalization(')
    end_idx = content.find('func main() {')
    if start_idx != -1 and end_idx != -1:
        content = content[:start_idx] + func_code + '\n\n' + content[end_idx:]
    else:
        content = content.replace('func main() {', func_code + '\n\nfunc main() {')

# 3. Use runFinalization in /finalize
if 'go runFinalization(' not in content:
    # Find the go func() block
    old_go_func = re.search(r'// Run finalization in background so the HTTP request doesn\'t timeout\n\t\tgo func\(\) \{.*?\n\t\t\}\(\)', content, re.DOTALL)
    if old_go_func:
        content = content.replace(old_go_func.group(0), '// Run finalization in background so the HTTP request doesn\'t timeout\n\t\tgo runFinalization(context.Background(), contestID, contest, teams, db, hub, redisClient)')

# 4. Save source code in /submit
if 'sourceCodeBytes' not in content:
    content = content.replace(
        'defer os.Remove(filePath)\n\n\t\tfmt.Println("Attempting to start sandbox for:", filePath)',
        'defer os.Remove(filePath)\n\t\tsourceCodeBytes, _ := os.ReadFile(filePath)\n\n\t\tfmt.Println("Attempting to start sandbox for:", filePath)'
    )
    content = content.replace(
        'targetURL := buildTargetURL(executionResult, protocol, port)\n\n\t\tfmt.Println("✓ Submission processed successfully',
        'targetURL := buildTargetURL(executionResult, protocol, port)\n\t\tsourceCodeMap.Store(targetURL, string(sourceCodeBytes))\n\n\t\tfmt.Println("✓ Submission processed successfully'
    )

# 5. Read source code in /stress-test
if 'sourceCodeStr :=' not in content:
    content = content.replace(
        'sr := store.NewSubmissionResultWithMode(\n\t\t\t\t\t\tsubmissionID, systemName, string(strategy), "", submitUserID,\n\t\t\t\t\t\tsc, perfMetrics, valResult,\n\t\t\t\t\t\tstring(judgingMode), req.ContestID, nil, seed,\n\t\t\t\t\t)',
        'sourceCodeStr := ""\n\t\t\t\t\tif src, ok := sourceCodeMap.LoadAndDelete(req.Target); ok {\n\t\t\t\t\t\tif str, valid := src.(string); valid {\n\t\t\t\t\t\t\tsourceCodeStr = str\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n\t\t\t\t\tsr := store.NewSubmissionResultWithMode(\n\t\t\t\t\t\tsubmissionID, systemName, string(strategy), "", submitUserID, sourceCodeStr,\n\t\t\t\t\t\tsc, perfMetrics, valResult,\n\t\t\t\t\t\tstring(judgingMode), req.ContestID, nil, seed,\n\t\t\t\t\t)'
    )
    content = content.replace(
        'res := store.NewSubmissionResultWithMode(\n\t\t\t\t\tsubmissionID, "Host-Demo", string(cfg.Strategy), "custom", userID,\n\t\t\t\t\tsc, perf, val, "demo", "", nil, cfg.Seed,\n\t\t\t\t)',
        'res := store.NewSubmissionResultWithMode(\n\t\t\t\t\tsubmissionID, "Host-Demo", string(cfg.Strategy), "custom", userID, "",\n\t\t\t\t\tsc, perf, val, "demo", "", nil, cfg.Seed,\n\t\t\t\t)'
    )

with open('cmd/main.go', 'w') as f:
    f.write(content)
