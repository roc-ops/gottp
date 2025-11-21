/**
 * GoTTP Editor - Main Application Logic
 */

class GottpEditor {
    constructor() {
        this.state = {
            template: '',
            inputs: { 'Default_Input': '' },
            variables: null,
            lookups: {},
            yangModules: {}, // name -> content
            compiledTemplate: null,
            templateCacheKey: null, // Cache key for faster template lookup
            lastResult: null,
            lastSourceMap: null, // Source map for visualization
            sourceMapDecorations: [], // Monaco decoration IDs for source map (input)
            outputSourceMapDecorations: [], // Monaco decoration IDs for source map (output)
            autoProcess: false, // Disabled by default to avoid issues with incomplete templates
            outputFormat: 'json',
            sourceMapsEnabled: false, // Default to off
            sourceMapColors: {
                // Input highlights
                matchedGutter: { color: '#89d185', opacity: 90 },
                unmatchedGutter: { color: '#f48771', opacity: 90 },
                groupHighlight: { color: '#ffa500', opacity: 15 },
                matchHighlight: { color: '#ffa500', opacity: 15 },
                variableHighlight: { color: '#007acc', opacity: 20 },
                hoverHighlight: { color: '#007acc', opacity: 25 },
                // Output highlights
                outputGroupHighlight: { color: '#ffa500', opacity: 15 }
            }
        };
        
        this.processDebounceTimer = null;
        this.processDebounceDelay = 500;
        
        this.init();
    }
    
    init() {
        // Wait for Monaco to be ready
        this.waitForMonaco().then(() => {
            this.loadWorkspaceFromStorage(); // Load options before setting up listeners
            this.setupEventListeners();
            this.setupKeyboardShortcuts();
            // Apply colors after everything is set up
            this.updateSourceMapColors();
        });
    }
    
    async waitForMonaco() {
        while (!inputEditor || !templateEditor) {
            await new Promise(resolve => setTimeout(resolve, 100));
        }
    }
    
    setupEventListeners() {
        // Menu buttons
        document.getElementById('process-button').addEventListener('click', () => this.process());
        document.getElementById('download-button').addEventListener('click', () => this.download());
        document.getElementById('auto-process').addEventListener('change', (e) => {
            this.state.autoProcess = e.target.checked;
        });
        document.getElementById('output-format').addEventListener('change', (e) => {
            this.state.outputFormat = e.target.value;
            if (this.state.lastResult) {
                this.displayOutput(this.state.lastResult);
            }
        });
        
        // Menu dropdowns
        this.setupMenuDropdowns();
        
        // Editor change listeners
        inputEditor.onDidChangeModelContent(() => {
            this.onInputChange();
        });
        
        templateEditor.onDidChangeModelContent(() => {
            this.onTemplateChange();
        });
        
        // Modal close buttons
        document.querySelectorAll('.modal-close').forEach(btn => {
            btn.addEventListener('click', (e) => {
                const modalId = e.target.getAttribute('data-modal');
                this.closeModal(modalId);
            });
        });
        
        // Click outside modal to close
        document.querySelectorAll('.modal').forEach(modal => {
            modal.addEventListener('click', (e) => {
                if (e.target === modal) {
                    this.closeModal(modal.id);
                }
            });
        });
        
        // Action menu items
        document.getElementById('process-btn').addEventListener('click', () => this.process());
        document.getElementById('clear-all-btn').addEventListener('click', () => this.clearAll());
        document.getElementById('load-example-btn').addEventListener('click', () => this.showExampleModal());
        
        // Config menu items
        document.getElementById('inputs-btn').addEventListener('click', () => this.showInputsModal());
        document.getElementById('variables-btn').addEventListener('click', () => this.showVariablesModal());
        document.getElementById('lookups-btn').addEventListener('click', () => this.showLookupsModal());
        document.getElementById('yang-modules-btn').addEventListener('click', () => this.showYANGModulesModal());
        
        // YANG module handlers
        this.setupYANGModuleHandlers();
        
        // File menu items
        document.getElementById('export-btn').addEventListener('click', () => this.exportConfig());
        document.getElementById('import-btn').addEventListener('click', () => this.importConfig());
        
        // Workspace menu items
        document.getElementById('save-workspace-btn').addEventListener('click', () => this.saveWorkspace());
        document.getElementById('load-workspace-btn').addEventListener('click', () => this.loadWorkspace());
        document.getElementById('manage-workspaces-btn').addEventListener('click', () => this.showWorkspaceManageModal());
        
        // Options menu
        document.getElementById('options-menu-btn').addEventListener('click', () => this.showOptionsModal());
        this.setupOptionsHandlers();
    }
    
    setupMenuDropdowns() {
        // Add click handlers for menu buttons to toggle dropdowns
        const menuButtons = [
            'file-menu-btn',
            'config-menu-btn',
            'actions-menu-btn',
            'workspace-menu-btn'
        ];
        
        menuButtons.forEach(btnId => {
            const btn = document.getElementById(btnId);
            if (btn) {
                btn.addEventListener('click', (e) => {
                    e.stopPropagation();
                    const menuItem = btn.closest('.menu-item');
                    const dropdown = menuItem?.querySelector('.dropdown-menu');
                    
                    // Close all other dropdowns
                    document.querySelectorAll('.dropdown-menu').forEach(menu => {
                        if (menu !== dropdown) {
                            menu.style.display = 'none';
                        }
                    });
                    
                    // Toggle this dropdown
                    if (dropdown) {
                        const isVisible = dropdown.style.display === 'block';
                        dropdown.style.display = isVisible ? 'none' : 'block';
                    }
                });
            }
        });
        
        // Close dropdowns when clicking outside
        document.addEventListener('click', (e) => {
            if (!e.target.closest('.menu-item')) {
                document.querySelectorAll('.dropdown-menu').forEach(menu => {
                    menu.style.display = 'none';
                });
            }
        });
        
        // Prevent dropdown from closing when clicking inside it
        // Use event delegation to handle dynamically added menus
        document.addEventListener('click', (e) => {
            if (e.target.closest('.dropdown-menu')) {
                e.stopPropagation();
            }
        });
    }
    
    onInputChange() {
        const value = inputEditor.getValue();
        this.state.inputs['Default_Input'] = value;
        this.saveStateToStorage();
        
        if (this.state.autoProcess) {
            this.debounceProcess();
        }
    }
    
    onTemplateChange() {
        const value = templateEditor.getValue();
        this.state.template = value;
        this.state.compiledTemplate = null; // Invalidate compiled template
        this.state.templateCacheKey = null;
        clearTemplateErrors();
        // Clear YANG validation markers
        if (templateEditor) {
            const model = templateEditor.getModel();
            monaco.editor.setModelMarkers(model, 'yang-validation', []);
        }
        this.saveStateToStorage();
        
        if (this.state.autoProcess) {
            this.debounceProcess();
        }
    }
    
    debounceProcess() {
        if (this.processDebounceTimer) {
            clearTimeout(this.processDebounceTimer);
        }
        
        this.processDebounceTimer = setTimeout(() => {
            this.process();
        }, this.processDebounceDelay);
    }
    
    async process() {
        const template = this.state.template.trim();
        
        if (!template) {
            this.showNotification('Template is empty', 'warning');
            return;
        }
        
        // Skip processing if template is too short (likely incomplete)
        if (template.length < 3) {
            return;
        }
        
        // Clear stats initially
        this.updateOutputStats(null, null);
        
        try {
            let compileTime = null;
            let executionTime = null;
            
            // Compile template if needed
            if (!this.state.compiledTemplate) {
                this.showNotification('Compiling template...', 'info');
                try {
                    const compileStart = performance.now();
                    const compileResult = await wasmBridge.compileTemplate(this.state.template);
                    compileTime = performance.now() - compileStart;
                    // Store both JSON and cache key for faster parsing
                    this.state.compiledTemplate = compileResult.compiledJSON || compileResult;
                    this.state.templateCacheKey = compileResult.cacheKey || this.state.template;
                } catch (compileError) {
                    // Clear compiled template on error so we retry next time
                    this.state.compiledTemplate = null;
                    this.state.templateCacheKey = null;
                    throw compileError;
                }
            }
            
            // Prepare inputs
            const inputsJSON = JSON.stringify(this.state.inputs);
            const varsJSON = this.state.variables ? JSON.stringify(this.state.variables) : null;
            
            // Prepare YANG modules JSON
            let yangModulesJSON = null;
            if (Object.keys(this.state.yangModules).length > 0) {
                yangModulesJSON = JSON.stringify({
                    Modules: this.state.yangModules
                });
            }
            
            // Parse (pass cache key for faster execution, enable source map if enabled in options)
            const parseStart = performance.now();
            const parseResult = await wasmBridge.parseTemplate(
                this.state.compiledTemplate,
                inputsJSON,
                varsJSON,
                yangModulesJSON,
                this.state.templateCacheKey || this.state.template,
                this.state.sourceMapsEnabled // Use state setting
            );
            executionTime = performance.now() - parseStart;
            
            this.state.lastResult = parseResult.data;
            this.state.lastSourceMap = parseResult.sourceMap;
            this.displayOutput(parseResult.data);
            this.updateOutputStats(compileTime, executionTime);
            
            // Display validation errors if any
            this.displayValidationErrors(parseResult.validationResults);
            
            // Visualize source map if available
            if (parseResult.sourceMap) {
                this.visualizeSourceMap(parseResult.sourceMap);
            } else {
                // Clear decorations if no source map
                if (this.state.sourceMapDecorations && this.state.sourceMapDecorations.length > 0) {
                    if (typeof clearSourceMapDecorations === 'function') {
                        clearSourceMapDecorations(this.state.sourceMapDecorations);
                    }
                    this.state.sourceMapDecorations = [];
                }
            }
            
            this.showNotification('Template processed successfully', 'success');
            
        } catch (error) {
            this.showNotification(`Processing failed: ${error.message}`, 'error');
            this.displayError(error.message);
            this.updateOutputStats(null, null);
            
            // Clear source map decorations on error
            if (this.state.sourceMapDecorations && this.state.sourceMapDecorations.length > 0) {
                if (typeof clearSourceMapDecorations === 'function') {
                    clearSourceMapDecorations(this.state.sourceMapDecorations);
                }
                this.state.sourceMapDecorations = [];
            }
            
            // Try to parse error for line numbers
            const errorMatch = error.message.match(/line (\d+)/i);
            if (errorMatch) {
                const line = parseInt(errorMatch[1]);
                setTemplateErrors([{
                    line: line,
                    column: 1,
                    message: error.message
                }]);
            }
        }
    }
    
    updateOutputStats(compileTime, executionTime) {
        const statsElement = document.getElementById('output-stats');
        if (!statsElement) return;
        
        if (compileTime === null && executionTime === null) {
            statsElement.textContent = '';
            return;
        }
        
        const parts = [];
        if (compileTime !== null) {
            parts.push(`compile: ${compileTime.toFixed(1)}ms`);
        }
        if (executionTime !== null) {
            parts.push(`execution: ${executionTime.toFixed(1)}ms`);
        }
        
        statsElement.textContent = parts.join(' • ');
    }
    
    async displayOutput(result) {
        const format = this.state.outputFormat;
        const outputEditor = getOutputEditor();
        const outputDiv = document.getElementById('output-display');
        
        // If output editor is not ready yet, wait for it
        if (!outputEditor) {
            // Fallback to innerHTML if Monaco isn't ready
            outputDiv.innerHTML = '<p>Loading editor...</p>';
            return;
        }
        
        try {
            const resultJSON = JSON.stringify(result);
            let formatted = '';
            let language = 'plaintext';
            
            switch (format) {
                case 'json':
                    formatted = await wasmBridge.formatJSON(resultJSON);
                    language = 'json';
                    break;
                case 'yaml':
                    formatted = await wasmBridge.formatYAML(resultJSON);
                    language = 'yaml';
                    break;
                case 'table':
                    const table = await wasmBridge.formatTable(resultJSON);
                    // For table format, hide Monaco editor and show HTML table
                    if (outputEditor) {
                        const editorNode = outputEditor.getDomNode();
                        editorNode.style.display = 'none';
                        // Store editor node reference so we can restore it
                        if (!editorNode.dataset.monacoEditor) {
                            editorNode.dataset.monacoEditor = 'true';
                        }
                    }
                    // Create a wrapper to hold both editor and table
                    let tableWrapper = outputDiv.querySelector('.table-wrapper');
                    if (!tableWrapper) {
                        tableWrapper = document.createElement('div');
                        tableWrapper.className = 'table-wrapper';
                        outputDiv.appendChild(tableWrapper);
                    }
                    tableWrapper.innerHTML = this.formatTableAsHTML(table);
                    tableWrapper.style.display = 'block';
                    return; // Exit early for table format
                case 'csv':
                    formatted = await wasmBridge.formatCSV(resultJSON);
                    language = 'plaintext';
                    break;
                default:
                    formatted = JSON.stringify(result, null, 2);
                    language = 'json';
            }
            
            // Show Monaco editor (in case it was hidden for table format)
            if (outputEditor) {
                outputEditor.getDomNode().style.display = 'block';
                // Hide table wrapper if it exists
                const tableWrapper = outputDiv.querySelector('.table-wrapper');
                if (tableWrapper) {
                    tableWrapper.style.display = 'none';
                }
            }
            
            // Update Monaco editor
            const model = outputEditor.getModel();
            if (model) {
                // Update language if changed
                monaco.editor.setModelLanguage(model, language);
                // Set value
                model.setValue(formatted);
                // Expand all regions by default for better visibility
                setTimeout(() => {
                    try {
                        outputEditor.getAction('editor.unfoldAll').run();
                    } catch (e) {
                        // Action might not be available, ignore
                    }
                    
                    // Apply source map decorations to output editor if available
                    if (this.state.lastSourceMap && typeof applySourceMapOutputDecorations === 'function') {
                        // Clear previous output decorations
                        if (this.state.outputSourceMapDecorations && this.state.outputSourceMapDecorations.length > 0) {
                            outputEditor.deltaDecorations(this.state.outputSourceMapDecorations, []);
                        }
                        const outputDecorations = applySourceMapOutputDecorations(this.state.lastSourceMap, result);
                        this.state.outputSourceMapDecorations = outputDecorations;
                    }
                }, 100);
            }
        } catch (error) {
            if (outputEditor) {
                outputEditor.getDomNode().style.display = 'block';
                const model = outputEditor.getModel();
                if (model) {
                    model.setValue(`Error formatting output: ${error.message}`);
                    monaco.editor.setModelLanguage(model, 'plaintext');
                }
            } else {
                outputDiv.innerHTML = `<pre class="error">Error formatting output: ${error.message}</pre>`;
            }
        }
    }
    
    formatTableAsHTML(table) {
        if (!table || table.length === 0) {
            return '<p>No data to display</p>';
        }
        
        let html = '<table style="width: 100%; border-collapse: collapse;">';
        
        // Header row
        if (table.length > 0) {
            html += '<thead><tr>';
            table[0].forEach(header => {
                html += `<th style="border: 1px solid var(--border-color); padding: 8px; text-align: left; background-color: var(--bg-secondary);">${this.escapeHtml(header)}</th>`;
            });
            html += '</tr></thead>';
        }
        
        // Data rows
        html += '<tbody>';
        for (let i = 1; i < table.length; i++) {
            html += '<tr>';
            table[i].forEach(cell => {
                html += `<td style="border: 1px solid var(--border-color); padding: 8px;">${this.escapeHtml(cell)}</td>`;
            });
            html += '</tr>';
        }
        html += '</tbody></table>';
        
        return html;
    }
    
    displayError(message) {
        const outputEditor = getOutputEditor();
        if (outputEditor) {
            const model = outputEditor.getModel();
            if (model) {
                model.setValue(`Error: ${message}`);
                monaco.editor.setModelLanguage(model, 'plaintext');
            }
        } else {
            const outputDiv = document.getElementById('output-display');
            outputDiv.innerHTML = `<pre class="error">${this.escapeHtml(message)}</pre>`;
        }
    }
    
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
    
    clearAll() {
        if (confirm('Clear all inputs, template, and output?')) {
            inputEditor.setValue('');
            templateEditor.setValue('');
            
            // Clear output editor
            const outputEditor = getOutputEditor();
            if (outputEditor) {
                const model = outputEditor.getModel();
                if (model) {
                    model.setValue('');
                }
                outputEditor.getDomNode().style.display = 'block';
            } else {
                document.getElementById('output-display').innerHTML = '';
            }
            
            this.state = {
                template: '',
                inputs: { 'Default_Input': '' },
                variables: null,
                lookups: {},
                compiledTemplate: null,
                templateCacheKey: null,
                lastResult: null,
                lastSourceMap: null,
                sourceMapDecorations: [],
                outputSourceMapDecorations: [],
                autoProcess: this.state.autoProcess,
                outputFormat: this.state.outputFormat
            };
            
            // Clear source map decorations
            if (typeof clearSourceMapDecorations === 'function' && this.state.sourceMapDecorations) {
                clearSourceMapDecorations(this.state.sourceMapDecorations);
            }
            
            // Clear output source map decorations (reuse outputEditor from above)
            if (outputEditor && this.state.outputSourceMapDecorations && this.state.outputSourceMapDecorations.length > 0) {
                outputEditor.deltaDecorations(this.state.outputSourceMapDecorations, []);
            }
            clearTemplateErrors();
            this.saveStateToStorage();
            this.showNotification('All cleared', 'success');
        }
    }
    
    // Modal Management
    showModal(modalId) {
        const modal = document.getElementById(modalId);
        if (modal) {
            modal.classList.add('active');
        }
    }
    
    closeModal(modalId) {
        const modal = document.getElementById(modalId);
        if (modal) {
            modal.classList.remove('active');
        }
    }
    
    // Examples
    showExampleModal() {
        const modal = document.getElementById('example-modal');
        const listDiv = document.getElementById('examples-list');
        
        listDiv.innerHTML = '';
        const examples = getAllExamples();
        
        Object.keys(examples).forEach(key => {
            const example = examples[key];
            const item = document.createElement('div');
            item.className = 'example-item';
            item.innerHTML = `
                <h3>${this.escapeHtml(example.name)}</h3>
                <p style="color: var(--text-secondary); margin: 8px 0;">${this.escapeHtml(example.description)}</p>
                <button class="button-primary" data-example="${key}">Load</button>
            `;
            
            item.querySelector('button').addEventListener('click', () => {
                this.loadExample(key);
                this.closeModal('example-modal');
            });
            
            listDiv.appendChild(item);
        });
        
        this.showModal('example-modal');
    }
    
    loadExample(exampleKey) {
        const example = getExample(exampleKey);
        if (!example) return;
        
        inputEditor.setValue(example.data);
        templateEditor.setValue(example.template);
        this.state.inputs['Default_Input'] = example.data;
        this.state.template = example.template;
        this.state.compiledTemplate = null;
        
        if (this.state.autoProcess) {
            this.process();
        }
        
        this.showNotification(`Loaded example: ${example.name}`, 'success');
    }
    
    // Inputs Management
    showInputsModal() {
        // For now, we'll use a simple single input
        // Advanced multi-input support can be added later
        this.showNotification('Multiple inputs feature coming soon', 'info');
    }
    
    // Variables Management
    showVariablesModal() {
        const modal = document.getElementById('variables-modal');
        const editor = document.getElementById('variables-editor');
        
        if (this.state.variables) {
            editor.value = JSON.stringify(this.state.variables, null, 2);
        } else {
            editor.value = '';
        }
        
        document.getElementById('save-variables-btn').onclick = () => {
            this.saveVariables();
        };
        
        document.getElementById('clear-variables-btn').onclick = () => {
            editor.value = '';
            this.state.variables = null;
            this.saveStateToStorage();
            this.showNotification('Variables cleared', 'success');
        };
        
        this.showModal('variables-modal');
    }
    
    saveVariables() {
        const editor = document.getElementById('variables-editor');
        const value = editor.value.trim();
        
        if (!value) {
            this.state.variables = null;
            this.saveStateToStorage();
            this.closeModal('variables-modal');
            this.showNotification('Variables cleared', 'success');
            return;
        }
        
        try {
            // Try to parse as JSON first
            this.state.variables = JSON.parse(value);
            this.saveStateToStorage();
            this.closeModal('variables-modal');
            this.showNotification('Variables saved', 'success');
            
            if (this.state.autoProcess) {
                this.process();
            }
        } catch (error) {
            this.showNotification(`Invalid JSON: ${error.message}`, 'error');
        }
    }
    
    // Lookups Management
    showLookupsModal() {
        this.showNotification('Lookup tables feature coming soon', 'info');
    }
    
    // Export/Import
    exportConfig() {
        const config = {
            template: this.state.template,
            inputs: this.state.inputs,
            variables: this.state.variables,
            lookups: this.state.lookups,
            version: '1.0'
        };
        
        const blob = new Blob([JSON.stringify(config, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = 'gottp-config.export';
        a.click();
        URL.revokeObjectURL(url);
        
        this.showNotification('Configuration exported', 'success');
    }
    
    importConfig() {
        const input = document.createElement('input');
        input.type = 'file';
        input.accept = '.export,.json';
        
        input.onchange = (e) => {
            const file = e.target.files[0];
            if (!file) return;
            
            const reader = new FileReader();
            reader.onload = (e) => {
                try {
                    const config = JSON.parse(e.target.result);
                    
                    if (config.template) {
                        templateEditor.setValue(config.template);
                        this.state.template = config.template;
                        this.state.compiledTemplate = null;
                    }
                    
                    if (config.inputs) {
                        this.state.inputs = config.inputs;
                        if (config.inputs['Default_Input']) {
                            inputEditor.setValue(config.inputs['Default_Input']);
                        }
                    }
                    
                    if (config.variables) {
                        this.state.variables = config.variables;
                    }
                    
                    if (config.lookups) {
                        this.state.lookups = config.lookups;
                    }
                    
                    this.saveStateToStorage();
                    this.showNotification('Configuration imported', 'success');
                    
                    if (this.state.autoProcess) {
                        this.process();
                    }
                } catch (error) {
                    this.showNotification(`Import failed: ${error.message}`, 'error');
                }
            };
            
            reader.readAsText(file);
        };
        
        input.click();
    }
    
    // Workspace Management
    saveWorkspace() {
        const name = prompt('Enter workspace name:');
        if (!name) return;
        
        const workspace = {
            name: name,
            template: this.state.template,
            inputs: this.state.inputs,
            variables: this.state.variables,
            lookups: this.state.lookups,
            timestamp: new Date().toISOString()
        };
        
        const workspaces = this.getWorkspaces();
        workspaces[name] = workspace;
        localStorage.setItem('gottp_workspaces', JSON.stringify(workspaces));
        
        this.showNotification(`Workspace "${name}" saved`, 'success');
    }
    
    loadWorkspace() {
        const workspaces = this.getWorkspaces();
        const names = Object.keys(workspaces);
        
        if (names.length === 0) {
            this.showNotification('No saved workspaces', 'warning');
            return;
        }
        
        const name = prompt(`Enter workspace name to load:\n\nAvailable: ${names.join(', ')}`);
        if (!name || !workspaces[name]) {
            return;
        }
        
        const workspace = workspaces[name];
        templateEditor.setValue(workspace.template || '');
        inputEditor.setValue(workspace.inputs?.['Default_Input'] || '');
        
        this.state.template = workspace.template || '';
        this.state.inputs = workspace.inputs || { 'Default_Input': '' };
        this.state.variables = workspace.variables || null;
        this.state.lookups = workspace.lookups || {};
        this.state.compiledTemplate = null;
        
        this.saveStateToStorage();
        this.showNotification(`Workspace "${name}" loaded`, 'success');
        
        if (this.state.autoProcess) {
            this.process();
        }
    }
    
    showWorkspaceManageModal() {
        const modal = document.getElementById('workspace-manage-modal');
        const listDiv = document.getElementById('workspaces-list');
        
        listDiv.innerHTML = '';
        const workspaces = this.getWorkspaces();
        
        if (Object.keys(workspaces).length === 0) {
            listDiv.innerHTML = '<p style="color: var(--text-secondary);">No saved workspaces</p>';
        } else {
            Object.keys(workspaces).forEach(name => {
                const workspace = workspaces[name];
                const item = document.createElement('div');
                item.className = 'workspace-item';
                item.innerHTML = `
                    <div style="display: flex; justify-content: space-between; align-items: center;">
                        <div>
                            <strong>${this.escapeHtml(name)}</strong>
                            <p style="color: var(--text-secondary); font-size: 12px; margin: 4px 0;">
                                ${new Date(workspace.timestamp).toLocaleString()}
                            </p>
                        </div>
                        <div>
                            <button class="btn-small btn-edit" data-workspace="${name}" data-action="load">Load</button>
                            <button class="btn-small btn-delete" data-workspace="${name}" data-action="delete">Delete</button>
                        </div>
                    </div>
                `;
                
                item.querySelector('[data-action="load"]').addEventListener('click', () => {
                    this.loadWorkspaceByName(name);
                    this.closeModal('workspace-manage-modal');
                });
                
                item.querySelector('[data-action="delete"]').addEventListener('click', () => {
                    if (confirm(`Delete workspace "${name}"?`)) {
                        this.deleteWorkspace(name);
                        this.showWorkspaceManageModal(); // Refresh
                    }
                });
                
                listDiv.appendChild(item);
            });
        }
        
        this.showModal('workspace-manage-modal');
    }
    
    loadWorkspaceByName(name) {
        const workspaces = this.getWorkspaces();
        if (!workspaces[name]) return;
        
        const workspace = workspaces[name];
        templateEditor.setValue(workspace.template || '');
        inputEditor.setValue(workspace.inputs?.['Default_Input'] || '');
        
        this.state.template = workspace.template || '';
        this.state.inputs = workspace.inputs || { 'Default_Input': '' };
        this.state.variables = workspace.variables || null;
        this.state.lookups = workspace.lookups || {};
        this.state.compiledTemplate = null;
        
        this.saveStateToStorage();
        this.showNotification(`Workspace "${name}" loaded`, 'success');
        
        if (this.state.autoProcess) {
            this.process();
        }
    }
    
    deleteWorkspace(name) {
        const workspaces = this.getWorkspaces();
        delete workspaces[name];
        localStorage.setItem('gottp_workspaces', JSON.stringify(workspaces));
        this.showNotification(`Workspace "${name}" deleted`, 'success');
    }
    
    getWorkspaces() {
        const stored = localStorage.getItem('gottp_workspaces');
        return stored ? JSON.parse(stored) : {};
    }
    
    // Storage
    saveStateToStorage() {
        // Also save options state separately
        localStorage.setItem('gottp-editor-options', JSON.stringify({
            sourceMapsEnabled: this.state.sourceMapsEnabled,
            sourceMapColors: this.state.sourceMapColors
        }));
        const state = {
            template: this.state.template,
            inputs: this.state.inputs,
            variables: this.state.variables,
            lookups: this.state.lookups,
            autoProcess: this.state.autoProcess,
            outputFormat: this.state.outputFormat
        };
        localStorage.setItem('gottp_state', JSON.stringify(state));
    }
    
    loadWorkspaceFromStorage() {
        // Load options state first
        const optionsJson = localStorage.getItem('gottp-editor-options');
        if (optionsJson) {
            try {
                const options = JSON.parse(optionsJson);
                if (options.sourceMapsEnabled !== undefined) {
                    this.state.sourceMapsEnabled = options.sourceMapsEnabled;
                }
                if (options.sourceMapColors) {
                    this.state.sourceMapColors = { ...this.state.sourceMapColors, ...options.sourceMapColors };
                }
                // Apply colors immediately
                this.updateSourceMapColors();
            } catch (e) {
                console.error('Failed to load options:', e);
            }
        }
        
        const stored = localStorage.getItem('gottp_state');
        if (!stored) return;
        
        try {
            const state = JSON.parse(stored);
            if (state.template) {
                templateEditor.setValue(state.template);
                this.state.template = state.template;
            }
            if (state.inputs && state.inputs['Default_Input']) {
                inputEditor.setValue(state.inputs['Default_Input']);
                this.state.inputs = state.inputs;
            }
            if (state.variables) {
                this.state.variables = state.variables;
            }
            if (state.lookups) {
                this.state.lookups = state.lookups;
            }
            if (state.autoProcess !== undefined) {
                this.state.autoProcess = state.autoProcess;
                document.getElementById('auto-process').checked = state.autoProcess;
            }
            if (state.outputFormat) {
                this.state.outputFormat = state.outputFormat;
                document.getElementById('output-format').value = state.outputFormat;
            }
        } catch (error) {
            console.error('Failed to load state from storage:', error);
        }
    }
    
    // Download
    download() {
        if (!this.state.lastResult) {
            this.showNotification('No results to download', 'warning');
            return;
        }
        
        const format = this.state.outputFormat;
        const resultJSON = JSON.stringify(this.state.lastResult);
        
        wasmBridge.formatJSON(resultJSON).then(formatted => {
            let content = '';
            let extension = 'json';
            let mimeType = 'application/json';
            
            switch (format) {
                case 'json':
                    content = formatted;
                    extension = 'json';
                    mimeType = 'application/json';
                    break;
                case 'yaml':
                    wasmBridge.formatYAML(resultJSON).then(yaml => {
                        this.downloadFile(yaml, 'yaml', 'text/yaml');
                    });
                    return;
                case 'csv':
                    wasmBridge.formatCSV(resultJSON).then(csv => {
                        this.downloadFile(csv, 'csv', 'text/csv');
                    });
                    return;
                case 'table':
                    content = formatted;
                    extension = 'json';
                    mimeType = 'application/json';
                    break;
            }
            
            this.downloadFile(content, extension, mimeType);
        });
    }
    
    downloadFile(content, extension, mimeType) {
        const blob = new Blob([content], { type: mimeType });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `gottp-output.${extension}`;
        a.click();
        URL.revokeObjectURL(url);
    }
    
    // Notifications
    showNotification(message, type = 'info') {
        const container = document.getElementById('notifications');
        const notification = document.createElement('div');
        notification.className = `notification ${type}`;
        notification.innerHTML = `<div class="notification-message">${this.escapeHtml(message)}</div>`;
        
        container.appendChild(notification);
        
        setTimeout(() => {
            notification.style.opacity = '0';
            notification.style.transition = 'opacity 0.3s';
            setTimeout(() => {
                notification.remove();
            }, 300);
        }, 3000);
    }
    
    // YANG Modules
    setupYANGModuleHandlers() {
        // File input - support multiple files
        document.getElementById('yang-file-input').addEventListener('change', (e) => {
            const files = Array.from(e.target.files);
            if (files.length === 0) {
                return;
            }
            
            // Load all files
            let loadedCount = 0;
            let errorCount = 0;
            const totalFiles = files.length;
            
            files.forEach((file) => {
                const reader = new FileReader();
                reader.onload = async (event) => {
                    try {
                        // Extract module name from content
                        const content = event.target.result;
                        const moduleNameMatch = content.match(/module\s+(\S+)/);
                        const moduleName = moduleNameMatch 
                            ? moduleNameMatch[1] 
                            : file.name.replace(/\.yang$/i, '');
                        
                        await this.loadYANGModuleFromContent(moduleName, content);
                        loadedCount++;
                        
                        // Show notification when all files are loaded
                        if (loadedCount + errorCount === totalFiles) {
                            if (errorCount === 0) {
                                this.showNotification(`Successfully loaded ${loadedCount} YANG module(s)`, 'success');
                            } else {
                                this.showNotification(`Loaded ${loadedCount} module(s), ${errorCount} failed`, 'warning');
                            }
                            // Reset file input so the same files can be selected again if needed
                            e.target.value = '';
                        }
                    } catch (error) {
                        errorCount++;
                        this.showNotification(`Failed to load ${file.name}: ${error.message}`, 'error');
                        
                        // Show notification when all files are processed
                        if (loadedCount + errorCount === totalFiles) {
                            if (loadedCount > 0) {
                                this.showNotification(`Loaded ${loadedCount} module(s), ${errorCount} failed`, 'warning');
                            }
                            e.target.value = '';
                        }
                    }
                };
                reader.onerror = () => {
                    errorCount++;
                    this.showNotification(`Failed to read file: ${file.name}`, 'error');
                    
                    // Show notification when all files are processed
                    if (loadedCount + errorCount === totalFiles) {
                        if (loadedCount > 0) {
                            this.showNotification(`Loaded ${loadedCount} module(s), ${errorCount} failed`, 'warning');
                        }
                        e.target.value = '';
                    }
                };
                reader.readAsText(file);
            });
        });
        
        // Load from file button
        document.getElementById('yang-load-file-btn').addEventListener('click', () => {
            document.getElementById('yang-file-input').click();
        });
        
        // Paste button
        document.getElementById('yang-paste-btn').addEventListener('click', () => {
            const container = document.getElementById('yang-paste-container');
            container.style.display = container.style.display === 'none' ? 'block' : 'none';
            document.getElementById('yang-url-input-container').style.display = 'none';
        });
        
        // Load from URL button
        document.getElementById('yang-load-url-btn').addEventListener('click', () => {
            const container = document.getElementById('yang-url-input-container');
            container.style.display = container.style.display === 'none' ? 'block' : 'none';
            document.getElementById('yang-paste-container').style.display = 'none';
        });
        
        // URL load button
        document.getElementById('yang-url-load-btn').addEventListener('click', async () => {
            const url = document.getElementById('yang-url-input').value.trim();
            if (!url) {
                this.showNotification('Please enter a URL', 'warning');
                return;
            }
            try {
                const response = await fetch(url);
                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}`);
                }
                const content = await response.text();
                // Extract module name from content
                const moduleNameMatch = content.match(/module\s+(\S+)/);
                const moduleName = moduleNameMatch ? moduleNameMatch[1] : url.split('/').pop().replace('.yang', '');
                this.loadYANGModuleFromContent(moduleName, content);
                document.getElementById('yang-url-input').value = '';
                document.getElementById('yang-url-input-container').style.display = 'none';
            } catch (error) {
                this.showNotification(`Failed to load YANG module from URL: ${error.message}`, 'error');
            }
        });
        
        // Paste load button
        document.getElementById('yang-paste-load-btn').addEventListener('click', () => {
            const content = document.getElementById('yang-paste-content').value.trim();
            if (!content) {
                this.showNotification('Please paste YANG module content', 'warning');
                return;
            }
            let moduleName = document.getElementById('yang-module-name').value.trim();
            if (!moduleName) {
                // Try to extract from content
                const moduleNameMatch = content.match(/module\s+(\S+)/);
                moduleName = moduleNameMatch ? moduleNameMatch[1] : 'unknown';
            }
            this.loadYANGModuleFromContent(moduleName, content);
            document.getElementById('yang-paste-content').value = '';
            document.getElementById('yang-module-name').value = '';
            document.getElementById('yang-paste-container').style.display = 'none';
        });
    }
    
    async loadYANGModuleFromContent(moduleName, content) {
        try {
            // Validate by trying to load it
            await wasmBridge.loadYANGModule(moduleName, content);
            this.state.yangModules[moduleName] = content;
            this.updateYANGModulesList();
            this.saveStateToStorage();
            this.showNotification(`YANG module '${moduleName}' loaded successfully`, 'success');
        } catch (error) {
            this.showNotification(`Failed to load YANG module '${moduleName}': ${error.message}`, 'error');
        }
    }
    
    showYANGModulesModal() {
        this.updateYANGModulesList();
        this.showModal('yang-modules-modal');
    }
    
    updateYANGModulesList() {
        const list = document.getElementById('yang-modules-list');
        const modules = Object.keys(this.state.yangModules);
        
        if (modules.length === 0) {
            list.innerHTML = '<div style="color: var(--text-secondary); text-align: center; padding: 20px;">No YANG modules loaded</div>';
            return;
        }
        
        list.innerHTML = modules.map(name => `
            <div style="display: flex; justify-content: space-between; align-items: center; padding: 10px; border-bottom: 1px solid var(--border-color);">
                <div>
                    <strong>${this.escapeHtml(name)}</strong>
                    <div style="color: var(--text-secondary); font-size: 12px; margin-top: 4px;">
                        ${this.state.yangModules[name].length} bytes
                    </div>
                </div>
                <button class="button-secondary" onclick="app.removeYANGModule('${this.escapeHtml(name)}')" style="padding: 4px 12px; font-size: 12px;">Remove</button>
            </div>
        `).join('');
    }
    
    removeYANGModule(moduleName) {
        delete this.state.yangModules[moduleName];
        this.updateYANGModulesList();
        this.saveStateToStorage();
        this.showNotification(`YANG module '${moduleName}' removed`, 'info');
    }
    
    displayValidationErrors(validationResults) {
        if (!validationResults || Object.keys(validationResults).length === 0) {
            // Clear any existing validation markers
            if (templateEditor) {
                const model = templateEditor.getModel();
                monaco.editor.setModelMarkers(model, 'yang-validation', []);
            }
            return;
        }
        
        // Count total errors and warnings
        let totalErrors = 0;
        let totalWarnings = 0;
        for (const result of Object.values(validationResults)) {
            if (result && result.Errors) {
                totalErrors += result.Errors.length;
            }
            if (result && result.Warnings) {
                totalWarnings += result.Warnings.length;
            }
        }
        
        // Set markers in editor
        if (typeof setYANGValidationErrors === 'function') {
            setYANGValidationErrors(validationResults);
        }
        
        // Show notification
        if (totalErrors > 0 || totalWarnings > 0) {
            const messages = [];
            if (totalErrors > 0) {
                messages.push(`${totalErrors} validation error${totalErrors > 1 ? 's' : ''}`);
            }
            if (totalWarnings > 0) {
                messages.push(`${totalWarnings} warning${totalWarnings > 1 ? 's' : ''}`);
            }
            this.showNotification(`YANG Validation: ${messages.join(', ')}`, totalErrors > 0 ? 'error' : 'warning');
        }
    }
    
    setupOptionsHandlers() {
        // Source maps toggle
        const sourceMapsCheckbox = document.getElementById('source-maps-enabled');
        if (sourceMapsCheckbox) {
            sourceMapsCheckbox.checked = this.state.sourceMapsEnabled;
            sourceMapsCheckbox.addEventListener('change', (e) => {
                this.state.sourceMapsEnabled = e.target.checked;
                const colorControls = document.getElementById('source-map-color-controls');
                if (colorControls) {
                    colorControls.style.display = e.target.checked ? 'block' : 'none';
                }
                this.saveStateToStorage();
                
                // If disabled, clear existing decorations and navigation data
                if (!e.target.checked) {
                    if (this.state.sourceMapDecorations && this.state.sourceMapDecorations.length > 0) {
                        if (typeof clearSourceMapDecorations === 'function') {
                            clearSourceMapDecorations(this.state.sourceMapDecorations);
                        }
                        this.state.sourceMapDecorations = [];
                    }
                    // Clear output decorations
                    if (this.state.outputSourceMapDecorations && this.state.outputSourceMapDecorations.length > 0) {
                        const outputEditor = getOutputEditor();
                        if (outputEditor) {
                            outputEditor.deltaDecorations(this.state.outputSourceMapDecorations, []);
                        }
                        this.state.outputSourceMapDecorations = [];
                    }
                    // Clear navigation click/hover decorations
                    const inputEditor = getInputEditor();
                    if (inputEditor && typeof window !== 'undefined') {
                        // Access the global variables from monaco-config.js
                        if (window.currentInputClickDecorations && window.currentInputClickDecorations.length > 0) {
                            inputEditor.deltaDecorations(window.currentInputClickDecorations, []);
                            window.currentInputClickDecorations.length = 0; // Clear array
                        }
                        if (window.currentInputHoverDecorations && window.currentInputHoverDecorations.length > 0) {
                            inputEditor.deltaDecorations(window.currentInputHoverDecorations, []);
                            window.currentInputHoverDecorations.length = 0; // Clear array
                        }
                    }
                    const outputEditor = getOutputEditor();
                    if (outputEditor && typeof window !== 'undefined') {
                        if (window.currentOutputHoverDecorations && window.currentOutputHoverDecorations.length > 0) {
                            outputEditor.deltaDecorations(window.currentOutputHoverDecorations, []);
                            window.currentOutputHoverDecorations.length = 0; // Clear array
                        }
                    }
                    // Clear navigation data to disable click handlers
                    if (typeof buildSourceMapNavigationData === 'function') {
                        buildSourceMapNavigationData(null, 'Default_Input');
                    }
                } else if (e.target.checked && this.state.lastSourceMap) {
                    // If enabled and we have a source map, visualize it
                    this.visualizeSourceMap(this.state.lastSourceMap);
                }
            });
        }
        
        // Color/opacity controls - Input
        const inputColorControls = [
            { id: 'matched-gutter', key: 'matchedGutter' },
            { id: 'unmatched-gutter', key: 'unmatchedGutter' },
            { id: 'group-highlight', key: 'groupHighlight' },
            { id: 'match-highlight', key: 'matchHighlight' },
            { id: 'variable-highlight', key: 'variableHighlight' },
            { id: 'hover-highlight', key: 'hoverHighlight' }
        ];
        
        // Color/opacity controls - Output
        const outputColorControls = [
            { id: 'output-group-highlight', key: 'outputGroupHighlight' }
        ];
        
        const colorControls = [...inputColorControls, ...outputColorControls];
        
        colorControls.forEach(control => {
            const colorInput = document.getElementById(`${control.id}-color`);
            const opacityInput = document.getElementById(`${control.id}-opacity`);
            const opacityValue = document.getElementById(`${control.id}-opacity-value`);
            
            if (colorInput && opacityInput && opacityValue) {
                // Initialize values
                const colors = this.state.sourceMapColors[control.key];
                colorInput.value = colors.color;
                opacityInput.value = colors.opacity;
                opacityValue.textContent = `${colors.opacity}%`;
                
                // Update on change
                const updateColor = () => {
                    const color = colorInput.value;
                    const opacity = parseInt(opacityInput.value);
                    this.state.sourceMapColors[control.key] = { color, opacity };
                    opacityValue.textContent = `${opacity}%`;
                    this.updateSourceMapColors();
                    this.saveStateToStorage();
                };
                
                colorInput.addEventListener('input', updateColor);
                opacityInput.addEventListener('input', updateColor);
            }
        });
        
        // Initialize color controls visibility
        const colorControlsDiv = document.getElementById('source-map-color-controls');
        if (colorControlsDiv) {
            colorControlsDiv.style.display = this.state.sourceMapsEnabled ? 'block' : 'none';
        }
    }
    
    updateSourceMapColors() {
        const colors = this.state.sourceMapColors;
        
        // Convert hex to RGB and apply opacity
        const hexToRgb = (hex) => {
            const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
            return result ? {
                r: parseInt(result[1], 16),
                g: parseInt(result[2], 16),
                b: parseInt(result[3], 16)
            } : null;
        };
        
        const applyColor = (selector, color, opacity) => {
            const rgb = hexToRgb(color);
            if (rgb) {
                const style = document.createElement('style');
                style.id = `source-map-${selector}-style`;
                const existing = document.getElementById(`source-map-${selector}-style`);
                if (existing) {
                    existing.remove();
                }
                style.textContent = `
                    .monaco-editor .source-map-${selector} {
                        background-color: rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, ${opacity / 100}) !important;
                    }
                `;
                document.head.appendChild(style);
            }
        };
        
        // Apply colors
        applyColor('matched-gutter', colors.matchedGutter.color, colors.matchedGutter.opacity);
        applyColor('unmatched-gutter', colors.unmatchedGutter.color, colors.unmatchedGutter.opacity);
        applyColor('group-highlight', colors.groupHighlight.color, colors.groupHighlight.opacity);
        applyColor('match-highlight', colors.matchHighlight.color, colors.matchHighlight.opacity);
        applyColor('variable-highlight', colors.variableHighlight.color, colors.variableHighlight.opacity);
        applyColor('hover-highlight', colors.hoverHighlight.color, colors.hoverHighlight.opacity);
        
        // Apply group highlight to match highlights in input editor (the brown/orange highlights)
        const groupRgb = hexToRgb(colors.groupHighlight.color);
        if (groupRgb) {
            const style = document.createElement('style');
            style.id = 'source-map-group-highlight-style';
            const existing = document.getElementById('source-map-group-highlight-style');
            if (existing) {
                existing.remove();
            }
            style.textContent = `
                .monaco-editor .source-map-match-highlight {
                    background-color: rgba(${groupRgb.r}, ${groupRgb.g}, ${groupRgb.b}, ${colors.groupHighlight.opacity / 100}) !important;
                }
            `;
            document.head.appendChild(style);
        }
        
        // Apply output group highlight to clickable keys in output (group names) - separate from input
        const outputGroupRgb = hexToRgb(colors.outputGroupHighlight.color);
        if (outputGroupRgb) {
            const style = document.createElement('style');
            style.id = 'source-map-clickable-key-style';
            const existing = document.getElementById('source-map-clickable-key-style');
            if (existing) {
                existing.remove();
            }
            style.textContent = `
                .monaco-editor .source-map-clickable-key {
                    background-color: rgba(${outputGroupRgb.r}, ${outputGroupRgb.g}, ${outputGroupRgb.b}, ${colors.outputGroupHighlight.opacity / 100}) !important;
                    border-bottom: 1px dashed rgba(${outputGroupRgb.r}, ${outputGroupRgb.g}, ${outputGroupRgb.b}, ${colors.outputGroupHighlight.opacity / 100 * 2}) !important;
                }
                .monaco-editor .source-map-clickable-key:hover {
                    background-color: rgba(${outputGroupRgb.r}, ${outputGroupRgb.g}, ${outputGroupRgb.b}, ${colors.outputGroupHighlight.opacity / 100 * 1.5}) !important;
                    border-bottom: 2px solid rgba(${outputGroupRgb.r}, ${outputGroupRgb.g}, ${outputGroupRgb.b}, ${colors.outputGroupHighlight.opacity / 100 * 2.8}) !important;
                }
            `;
            document.head.appendChild(style);
        }
        
        // Also update glyph margin markers (they use different selectors)
        const applyGlyphColor = (selector, color, opacity) => {
            const rgb = hexToRgb(color);
            if (rgb) {
                const style = document.createElement('style');
                style.id = `source-map-glyph-${selector}-style`;
                const existing = document.getElementById(`source-map-glyph-${selector}-style`);
                if (existing) {
                    existing.remove();
                }
                style.textContent = `
                    .monaco-editor .monaco-glyph-margin .source-map-${selector} {
                        background-color: rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, ${opacity / 100}) !important;
                    }
                    .monaco-editor [class*="source-map-${selector}"] {
                        background-color: rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, ${opacity / 100}) !important;
                    }
                `;
                document.head.appendChild(style);
            }
        };
        
        applyGlyphColor('matched-gutter', colors.matchedGutter.color, colors.matchedGutter.opacity);
        applyGlyphColor('unmatched-gutter', colors.unmatchedGutter.color, colors.unmatchedGutter.opacity);
    }
    
    showOptionsModal() {
        this.showModal('options-modal');
        // Update controls to reflect current state
        const sourceMapsCheckbox = document.getElementById('source-maps-enabled');
        if (sourceMapsCheckbox) {
            sourceMapsCheckbox.checked = this.state.sourceMapsEnabled;
        }
    }
    
    // Visualize source map
    visualizeSourceMap(sourceMap) {
        if (!sourceMap || !sourceMap.Inputs) {
            return;
        }
        
        // Clear previous decorations
        if (this.state.sourceMapDecorations && this.state.sourceMapDecorations.length > 0) {
            if (typeof clearSourceMapDecorations === 'function') {
                clearSourceMapDecorations(this.state.sourceMapDecorations);
            }
            this.state.sourceMapDecorations = [];
        }
        
        // Get input editor
        const inputName = 'Default_Input';
        const inputSourceMap = sourceMap.Inputs[inputName];
        if (!inputSourceMap || !inputSourceMap.Lines) {
            return;
        }
        
        // Build navigation data structure
        if (typeof buildSourceMapNavigationData === 'function') {
            buildSourceMapNavigationData(sourceMap, inputName);
        }
        
        // Apply Monaco decorations for visualization
        if (typeof applySourceMapDecorations === 'function') {
            const decorationIds = applySourceMapDecorations(sourceMap, inputName);
            this.state.sourceMapDecorations = decorationIds;
        } else {
            console.warn('Source map decorations function not available');
        }
    }
    
    // Keyboard Shortcuts
    setupKeyboardShortcuts() {
        document.addEventListener('keydown', (e) => {
            const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
            const ctrlKey = isMac ? e.metaKey : e.ctrlKey;
            
            // Ctrl/Cmd + Enter: Process
            if (ctrlKey && e.key === 'Enter') {
                e.preventDefault();
                this.process();
            }
            
            // Ctrl/Cmd + K: Clear all
            if (ctrlKey && e.key === 'k') {
                e.preventDefault();
                this.clearAll();
            }
            
            // Ctrl/Cmd + L: Load example
            if (ctrlKey && e.key === 'l') {
                e.preventDefault();
                this.showExampleModal();
            }
            
            // Escape: Close modals
            if (e.key === 'Escape') {
                document.querySelectorAll('.modal.active').forEach(modal => {
                    this.closeModal(modal.id);
                });
            }
        });
    }
}

// Initialize app when DOM is ready
let app = null;
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
        app = new GottpEditor();
    });
} else {
    app = new GottpEditor();
}

