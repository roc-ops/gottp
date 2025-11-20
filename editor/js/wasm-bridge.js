/**
 * WASM Bridge for GoTTP
 * Loads and initializes the Go WebAssembly module
 */

class GottpWasmBridge {
    constructor() {
        this.go = null;
        this.gottp = null;
        this.initialized = false;
        this.initPromise = null;
    }

    /**
     * Initialize the WASM module
     */
    async init() {
        if (this.initPromise) {
            return this.initPromise;
        }

        this.initPromise = (async () => {
            try {
                // Load wasm_exec.js
                if (typeof Go === 'undefined') {
                    try {
                        await this.loadScript('wasm_exec.js');
                    } catch (error) {
                        throw new Error('Failed to load wasm_exec.js. Please run "make build" to generate it.');
                    }
                }

                // Load WASM module
                let wasmResponse;
                try {
                    wasmResponse = await fetch('gottp.wasm');
                    if (!wasmResponse.ok) {
                        throw new Error(`Failed to load gottp.wasm: ${wasmResponse.status} ${wasmResponse.statusText}`);
                    }
                } catch (error) {
                    throw new Error('Failed to load gottp.wasm. Please run "make build" to generate it.');
                }
                const wasmBytes = await wasmResponse.arrayBuffer();

                // Initialize Go runtime
                this.go = new Go();
                const result = await WebAssembly.instantiate(wasmBytes, this.go.importObject);
                
                // Run Go program in a separate goroutine
                this.go.run(result.instance);

                // Wait for gottp to be available
                await this.waitForGottp();

                this.initialized = true;
                console.log('GoTTP WASM module initialized');
            } catch (error) {
                console.error('Failed to initialize WASM module:', error);
                throw error;
            }
        })();

        return this.initPromise;
    }

    /**
     * Load a script dynamically
     */
    loadScript(src) {
        return new Promise((resolve, reject) => {
            const script = document.createElement('script');
            script.src = src;
            script.onload = resolve;
            script.onerror = reject;
            document.head.appendChild(script);
        });
    }

    /**
     * Wait for gottp global to be available
     */
    waitForGottp(maxAttempts = 100, delay = 50) {
        return new Promise((resolve, reject) => {
            let attempts = 0;
            const check = () => {
                if (window.gottp) {
                    this.gottp = window.gottp;
                    resolve();
                } else if (attempts < maxAttempts) {
                    attempts++;
                    setTimeout(check, delay);
                } else {
                    reject(new Error('gottp global not available after initialization'));
                }
            };
            check();
        });
    }

    /**
     * Compile a TTP template
     * Returns both the compiled JSON and cache key for faster parsing
     */
    async compileTemplate(templateText) {
        if (!this.initialized) {
            await this.init();
        }

        try {
            const result = this.gottp.compileTemplate(templateText);
            const resultObj = this.jsValueToObject(result);
            
            if (resultObj.error) {
                throw new Error(resultObj.error);
            }

            // Return both JSON and cache key
            return {
                compiledJSON: resultObj.result,
                cacheKey: resultObj.cacheKey || templateText
            };
        } catch (error) {
            throw new Error(`Template compilation failed: ${error.message}`);
        }
    }

    /**
     * Parse input data using a compiled template
     * @param {string|object} compiledTemplate - Compiled template JSON string or object with {compiledJSON, cacheKey}
     * @param {string} inputsJSON - Inputs JSON
     * @param {string|null} varsJSON - Variables JSON (optional)
     * @param {string|null} yangModulesJSON - YANG modules JSON (optional)
     * @returns {Promise<{data: *, validationResults: *}>}
     */
    async parseTemplate(compiledTemplate, inputsJSON, varsJSON = null, yangModulesJSON = null, cacheKey = null) {
        if (!this.initialized) {
            await this.init();
        }

        try {
            // Handle both old format (string) and new format (object with cacheKey)
            let compiledTemplateJSON;
            let templateCacheKey = cacheKey;
            
            if (typeof compiledTemplate === 'string') {
                compiledTemplateJSON = compiledTemplate;
            } else if (compiledTemplate && compiledTemplate.compiledJSON) {
                compiledTemplateJSON = compiledTemplate.compiledJSON;
                templateCacheKey = compiledTemplate.cacheKey || templateCacheKey;
            } else {
                compiledTemplateJSON = compiledTemplate;
            }

            const varsJSONStr = varsJSON || 'null';
            const yangModulesJSONStr = yangModulesJSON || 'null';
            const cacheKeyStr = templateCacheKey || 'null';
            
            const result = this.gottp.parseTemplate(
                compiledTemplateJSON,
                inputsJSON,
                varsJSONStr,
                yangModulesJSONStr,
                cacheKeyStr
            );
            const resultObj = this.jsValueToObject(result);
            
            if (resultObj.error) {
                throw new Error(resultObj.error);
            }

            const data = JSON.parse(resultObj.result);
            const validationResults = resultObj.validationResults 
                ? JSON.parse(resultObj.validationResults) 
                : {};

            return {
                data: data,
                validationResults: validationResults
            };
        } catch (error) {
            throw new Error(`Template parsing failed: ${error.message}`);
        }
    }

    /**
     * Load a YANG module from string content
     * @param {string} moduleName - Name of the YANG module
     * @param {string} moduleContent - YANG module content
     * @returns {Promise<string>} JSON string of YANGModules structure
     */
    async loadYANGModule(moduleName, moduleContent) {
        if (!this.initialized) {
            await this.init();
        }

        try {
            const result = this.gottp.loadYANGModule(moduleName, moduleContent);
            const resultObj = this.jsValueToObject(result);
            
            if (resultObj.error) {
                throw new Error(resultObj.error);
            }

            return resultObj.result;
        } catch (error) {
            throw new Error(`YANG module loading failed: ${error.message}`);
        }
    }

    /**
     * Format data as JSON
     */
    async formatJSON(dataJSON) {
        if (!this.initialized) {
            await this.init();
        }

        try {
            const result = this.gottp.formatJSON(dataJSON);
            const resultObj = this.jsValueToObject(result);
            
            if (resultObj.error) {
                throw new Error(resultObj.error);
            }

            return resultObj.result;
        } catch (error) {
            throw new Error(`JSON formatting failed: ${error.message}`);
        }
    }

    /**
     * Format data as YAML
     */
    async formatYAML(dataJSON) {
        if (!this.initialized) {
            await this.init();
        }

        try {
            const result = this.gottp.formatYAML(dataJSON);
            const resultObj = this.jsValueToObject(result);
            
            if (resultObj.error) {
                throw new Error(resultObj.error);
            }

            return resultObj.result;
        } catch (error) {
            throw new Error(`YAML formatting failed: ${error.message}`);
        }
    }

    /**
     * Format data as table (returns array of arrays)
     */
    async formatTable(dataJSON) {
        if (!this.initialized) {
            await this.init();
        }

        try {
            const result = this.gottp.formatTable(dataJSON);
            const resultObj = this.jsValueToObject(result);
            
            if (resultObj.error) {
                throw new Error(resultObj.error);
            }

            return JSON.parse(resultObj.result);
        } catch (error) {
            throw new Error(`Table formatting failed: ${error.message}`);
        }
    }

    /**
     * Format data as CSV
     */
    async formatCSV(dataJSON) {
        if (!this.initialized) {
            await this.init();
        }

        try {
            const result = this.gottp.formatCSV(dataJSON);
            const resultObj = this.jsValueToObject(result);
            
            if (resultObj.error) {
                throw new Error(resultObj.error);
            }

            return resultObj.result;
        } catch (error) {
            throw new Error(`CSV formatting failed: ${error.message}`);
        }
    }

    /**
     * Convert JS value to JavaScript object
     */
    jsValueToObject(jsValue) {
        if (!jsValue || typeof jsValue !== 'object') {
            return jsValue;
        }

        const obj = {};
        const keys = Object.keys(jsValue);
        for (const key of keys) {
            const value = jsValue[key];
            if (value && typeof value === 'object' && value.constructor && value.constructor.name === 'Object') {
                obj[key] = this.jsValueToObject(value);
            } else if (value === null || value === undefined) {
                obj[key] = null;
            } else {
                obj[key] = value;
            }
        }
        return obj;
    }
}

// Create global instance
const wasmBridge = new GottpWasmBridge();

// Initialize on page load
window.addEventListener('DOMContentLoaded', async () => {
    try {
        await wasmBridge.init();
        // Hide loading screen and show app
        document.getElementById('loading-screen').style.display = 'none';
        document.getElementById('app').style.display = 'flex';
    } catch (error) {
        console.error('Failed to initialize WASM:', error);
        const loadingScreen = document.getElementById('loading-screen');
        loadingScreen.innerHTML = `
            <div class="loading-content">
                <p style="color: var(--error-color); font-size: 18px; font-weight: bold; margin-bottom: 20px;">
                    Failed to load GoTTP WebAssembly module
                </p>
                <p style="color: var(--text-secondary); margin: 10px 0;">
                    ${error.message}
                </p>
                <div style="margin-top: 30px; padding: 20px; background-color: var(--bg-secondary); border-radius: 4px; max-width: 600px;">
                    <p style="color: var(--text-primary); margin-bottom: 10px; font-weight: bold;">To fix this issue:</p>
                    <ol style="color: var(--text-secondary); text-align: left; margin-left: 20px;">
                        <li style="margin: 8px 0;">Navigate to the editor directory: <code style="background: var(--bg-primary); padding: 2px 6px; border-radius: 3px;">cd editor</code></li>
                        <li style="margin: 8px 0;">Build the WASM module: <code style="background: var(--bg-primary); padding: 2px 6px; border-radius: 3px;">make build</code></li>
                        <li style="margin: 8px 0;">Refresh this page</li>
                    </ol>
                    <p style="color: var(--text-secondary); margin-top: 15px; font-size: 12px;">
                        This will generate <code style="background: var(--bg-primary); padding: 2px 6px; border-radius: 3px;">gottp.wasm</code> and <code style="background: var(--bg-primary); padding: 2px 6px; border-radius: 3px;">wasm_exec.js</code> files.
                    </p>
                </div>
            </div>
        `;
    }
});

