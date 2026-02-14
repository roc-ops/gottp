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

            // Return both JSON and cache key, plus warnings
            let warnings = [];
            if (resultObj.warnings) {
                // Check if it's already an array (from jsValueToObject conversion)
                if (Array.isArray(resultObj.warnings)) {
                    warnings = resultObj.warnings;
                } else if (typeof resultObj.warnings === 'string') {
                    // It's a JSON string, parse it
                    try {
                        warnings = JSON.parse(resultObj.warnings);
                    } catch (e) {
                        // If parsing fails, ignore warnings
                    }
                }
            }
            
            return {
                compiledJSON: resultObj.result,
                cacheKey: resultObj.cacheKey || templateText,
                warnings: warnings
            };
        } catch (error) {
            throw new Error(`Template compilation failed: ${error.message}`);
        }
    }

    /**
     * Compile a TTP template with compile-time options
     * Supports pre-registered function sets via options.functionSet name
     * @param {string} templateText - The TTP template text
     * @param {object|null} options - Compile options (optional). Supports { functionSet: "name" }
     * @returns {Promise<{compiledJSON: string, cacheKey: string, warnings: Array}>}
     */
    async compileTemplateWithOptions(templateText, options = null) {
        if (!this.initialized) {
            await this.init();
        }

        try {
            const optionsJSON = options ? JSON.stringify(options) : 'null';
            const result = this.gottp.compileTemplateWithOptions(templateText, optionsJSON);
            const resultObj = this.jsValueToObject(result);

            if (resultObj.error) {
                throw new Error(resultObj.error);
            }

            // Parse warnings
            let warnings = [];
            if (resultObj.warnings) {
                if (Array.isArray(resultObj.warnings)) {
                    warnings = resultObj.warnings;
                } else if (typeof resultObj.warnings === 'string') {
                    try {
                        warnings = JSON.parse(resultObj.warnings);
                    } catch (e) {
                        // If parsing fails, ignore warnings
                    }
                }
            }

            return {
                compiledJSON: resultObj.result,
                cacheKey: resultObj.cacheKey || templateText,
                warnings: warnings
            };
        } catch (error) {
            throw new Error(`Template compilation with options failed: ${error.message}`);
        }
    }

    /**
     * List all registered function set names
     * @returns {Promise<Array<string>>} Array of registered function set names
     */
    async listFunctionSets() {
        if (!this.initialized) {
            await this.init();
        }

        try {
            const result = this.gottp.listFunctionSets();
            const resultObj = this.jsValueToObject(result);

            if (resultObj.error) {
                throw new Error(resultObj.error);
            }

            return JSON.parse(resultObj.result);
        } catch (error) {
            throw new Error(`Listing function sets failed: ${error.message}`);
        }
    }

    /**
     * Parse input data using a compiled template
     * @param {string|object} compiledTemplate - Compiled template JSON string or object with {compiledJSON, cacheKey}
     * @param {string} inputsJSON - Inputs JSON
     * @param {string|null} varsJSON - Variables JSON (optional)
     * @param {string|null} yangModulesJSON - YANG modules JSON (optional)
     * @param {string|null} cacheKey - Cache key for faster lookup (optional)
     * @param {boolean} enableSourceMap - Enable source map collection (optional)
     * @param {string|null} lookupsJSON - Lookups JSON (optional, map of named lookup tables)
     * @returns {Promise<{data: *, validationResults: *, sourceMap: *}>}
     */
    async parseTemplate(compiledTemplate, inputsJSON, varsJSON = null, yangModulesJSON = null, cacheKey = null, enableSourceMap = false, lookupsJSON = null) {
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
            const enableSourceMapStr = enableSourceMap ? 'true' : 'null';
            
            const lookupsJSONStr = lookupsJSON || 'null';

            const result = this.gottp.parseTemplate(
                compiledTemplateJSON,
                inputsJSON,
                varsJSONStr,
                yangModulesJSONStr,
                cacheKeyStr,
                enableSourceMapStr,
                lookupsJSONStr
            );
            const resultObj = this.jsValueToObject(result);
            
            if (resultObj.error) {
                throw new Error(resultObj.error);
            }

            const data = JSON.parse(resultObj.result);
            const validationResults = resultObj.validationResults 
                ? JSON.parse(resultObj.validationResults) 
                : {};
            const sourceMap = resultObj.sourceMap 
                ? JSON.parse(resultObj.sourceMap) 
                : null;

            return {
                data: data,
                validationResults: validationResults,
                sourceMap: sourceMap
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
     * Load a named lookup table from JSON data
     * @param {string} name - Name for the lookup table
     * @param {string} jsonData - JSON string of the lookup table (object of objects)
     * @returns {Promise<string>} JSON string of the lookup map (name -> table)
     */
    async loadLookupFromJSON(name, jsonData) {
        if (!this.initialized) {
            await this.init();
        }

        try {
            const result = this.gottp.loadLookupFromJSON(name, jsonData);
            const resultObj = this.jsValueToObject(result);

            if (resultObj.error) {
                throw new Error(resultObj.error);
            }

            return resultObj.result;
        } catch (error) {
            throw new Error(`Lookup loading from JSON failed: ${error.message}`);
        }
    }

    /**
     * Load a named lookup table from YAML data
     * @param {string} name - Name for the lookup table
     * @param {string} yamlData - YAML string of the lookup table
     * @returns {Promise<string>} JSON string of the lookup map (name -> table)
     */
    async loadLookupFromYAML(name, yamlData) {
        if (!this.initialized) {
            await this.init();
        }

        try {
            const result = this.gottp.loadLookupFromYAML(name, yamlData);
            const resultObj = this.jsValueToObject(result);

            if (resultObj.error) {
                throw new Error(resultObj.error);
            }

            return resultObj.result;
        } catch (error) {
            throw new Error(`Lookup loading from YAML failed: ${error.message}`);
        }
    }

    /**
     * Load a named lookup table from CSV data
     * @param {string} name - Name for the lookup table
     * @param {string} csvData - CSV string of the lookup table
     * @param {string} keyColumn - Column to use as lookup key (optional, defaults to first column)
     * @returns {Promise<string>} JSON string of the lookup map (name -> table)
     */
    async loadLookupFromCSV(name, csvData, keyColumn = '') {
        if (!this.initialized) {
            await this.init();
        }

        try {
            const result = this.gottp.loadLookupFromCSV(name, csvData, keyColumn);
            const resultObj = this.jsValueToObject(result);

            if (resultObj.error) {
                throw new Error(resultObj.error);
            }

            return resultObj.result;
        } catch (error) {
            throw new Error(`Lookup loading from CSV failed: ${error.message}`);
        }
    }

    /**
     * Load multiple named lookup tables from JSON data
     * @param {string} jsonData - JSON string containing multiple lookup tables (nested object)
     * @returns {Promise<string>} JSON string of the lookups map
     */
    async loadLookupsFromJSON(jsonData) {
        if (!this.initialized) {
            await this.init();
        }

        try {
            const result = this.gottp.loadLookupsFromJSON(jsonData);
            const resultObj = this.jsValueToObject(result);

            if (resultObj.error) {
                throw new Error(resultObj.error);
            }

            return resultObj.result;
        } catch (error) {
            throw new Error(`Lookups loading from JSON failed: ${error.message}`);
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
        // Use Object.getOwnPropertyNames to get all properties, including non-enumerable ones
        const keys = Object.getOwnPropertyNames(jsValue);
        for (const key of keys) {
            // Skip internal properties
            if (key.startsWith('_') || key === 'constructor' || key === 'prototype') {
                continue;
            }
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

