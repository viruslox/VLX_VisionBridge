with open('internal/engine/connector.go', 'r') as f:
    content = f.read()

target = """							// Copy config
							newCfg := *pm.config
							pm.mu.Unlock()

							pm.mu.Lock()
							pm.config = &newCfg
							pm.mu.Unlock()

							pm.UpdateFilter(&newCfg)
							return"""

replacement = """							// Copy config
							newCfg := *pm.config
							pm.mu.Unlock()

							// UPDATE STATE IN MEMORY WITHOUT RESTARTING FFMPEG
							pm.mu.Lock()
							pm.config = &newCfg
							pm.mu.Unlock()

							pm.UpdateFilter(&newCfg)
							return"""

with open('internal/engine/connector.go', 'w') as f:
    f.write(content.replace(target, replacement))
