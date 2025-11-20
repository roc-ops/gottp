/**
 * Monaco Editor Configuration for GoTTP Editor
 * Sets up Monaco Editor instances and TTP syntax highlighting
 */

let inputEditor = null;
let templateEditor = null;
let outputEditor = null;

// TTP Language Definition - Simplified to avoid infinite recursion
const ttpLanguageDefinition = {
    id: 'ttp',
    extensions: ['.ttp', '.txt'],
    aliases: ['TTP', 'Template Text Parser'],
    mimetypes: ['text/ttp'],
    
    tokenizer: {
        root: [
            // Complete comments only
            [/<!--[\s\S]*?-->/, 'comment'],
            
            // Complete variable syntax only
            [/{{[^}]*}}/, 'variable'],
            
            // Complete closing tags only
            [/<\/template>/, 'keyword'],
            [/<\/group>/, 'keyword'],
            [/<\/input>/, 'keyword'],
            [/<\/output>/, 'keyword'],
            [/<\/vars>/, 'keyword'],
            [/<\/lookup>/, 'keyword'],
            [/<\/macro>/, 'keyword'],
            
            // Complete opening tags only (must have closing >)
            [/<template[^>]*>/, 'keyword'],
            [/<group[^>]*>/, 'keyword'],
            [/<input[^>]*>/, 'keyword'],
            [/<output[^>]*>/, 'keyword'],
            [/<vars[^>]*>/, 'keyword'],
            [/<lookup[^>]*>/, 'keyword'],
            [/<macro[^>]*>/, 'keyword'],
            [/<extend[^>]*\/?>/, 'keyword'],
            
            // Self-closing tags
            [/<[a-zA-Z_][a-zA-Z0-9_]*\s*\/>/, 'keyword'],
            
            // Everything else (including incomplete tags like just "<")
            [/./, 'text']
        ]
    }
};

// TTP Auto-completion
const ttpCompletionProvider = {
    provideCompletionItems: (model, position) => {
        const suggestions = [
            // Template tags
            {
                label: 'template',
                kind: monaco.languages.CompletionItemKind.Keyword,
                insertText: '<template name="${1:template_name}">\n\t$0\n</template>',
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                documentation: 'Template container tag'
            },
            {
                label: 'group',
                kind: monaco.languages.CompletionItemKind.Keyword,
                insertText: '<group name="${1:group_name}">\n\t$0\n</group>',
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                documentation: 'Group pattern matching tag'
            },
            {
                label: 'input',
                kind: monaco.languages.CompletionItemKind.Keyword,
                insertText: '<input load="text">\n\t$0\n</input>',
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                documentation: 'Input data source configuration'
            },
            {
                label: 'vars',
                kind: monaco.languages.CompletionItemKind.Keyword,
                insertText: '<vars>\n\t$0\n</vars>',
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                documentation: 'Template variables'
            },
            {
                label: 'lookup',
                kind: monaco.languages.CompletionItemKind.Keyword,
                insertText: '<lookup name="${1:lookup_name}" load="yaml">\n\t$0\n</lookup>',
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                documentation: 'Lookup table definition'
            },
            
            // Variable syntax
            {
                label: 'variable',
                kind: monaco.languages.CompletionItemKind.Variable,
                insertText: '{{ ${1:variable_name} }}',
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                documentation: 'Template variable'
            },
            
            // Common functions
            {
                label: 'to_int',
                kind: monaco.languages.CompletionItemKind.Function,
                insertText: '{{ ${1:value} | to_int }}',
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                documentation: 'Convert to integer'
            },
            {
                label: 'to_float',
                kind: monaco.languages.CompletionItemKind.Function,
                insertText: '{{ ${1:value} | to_float }}',
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                documentation: 'Convert to float'
            },
            {
                label: 'upper',
                kind: monaco.languages.CompletionItemKind.Function,
                insertText: '{{ ${1:value} | upper }}',
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                documentation: 'Convert to uppercase'
            },
            {
                label: 'lower',
                kind: monaco.languages.CompletionItemKind.Function,
                insertText: '{{ ${1:value} | lower }}',
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                documentation: 'Convert to lowercase'
            },
            {
                label: 'strip',
                kind: monaco.languages.CompletionItemKind.Function,
                insertText: '{{ ${1:value} | strip }}',
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                documentation: 'Strip whitespace'
            },
            {
                label: 'split',
                kind: monaco.languages.CompletionItemKind.Function,
                insertText: '{{ ${1:value} | split("${2:delimiter}") }}',
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                documentation: 'Split string by delimiter'
            },
            {
                label: 're',
                kind: monaco.languages.CompletionItemKind.Function,
                insertText: '{{ ${1:value} | re("${2:pattern}") }}',
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                documentation: 'Regular expression matching'
            },
            {
                label: 'lookup',
                kind: monaco.languages.CompletionItemKind.Function,
                insertText: '{{ ${1:value} | lookup("${2:table_name}", "${3:key}") }}',
                insertTextRules: monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet,
                documentation: 'Lookup value in lookup table'
            }
        ];
        
        return { suggestions };
    }
};

/**
 * Initialize Monaco Editor
 */
async function initMonacoEditor() {
    // Configure Monaco loader
    require.config({ paths: { vs: 'https://cdn.jsdelivr.net/npm/monaco-editor@0.45.0/min/vs' } });
    
    return new Promise((resolve) => {
        require(['vs/editor/editor.main'], () => {
            // Register TTP language
            monaco.languages.register({ id: 'ttp' });
            monaco.languages.setMonarchTokensProvider('ttp', ttpLanguageDefinition);
            monaco.languages.registerCompletionItemProvider('ttp', ttpCompletionProvider);
            
            // Create input editor
            inputEditor = monaco.editor.create(document.getElementById('input-editor'), {
                value: '',
                language: 'plaintext',
                theme: 'vs-dark',
                fontSize: 13,
                fontFamily: 'Consolas, Monaco, "Courier New", monospace',
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                wordWrap: 'on',
                automaticLayout: true
            });
            
            // Create template editor
            templateEditor = monaco.editor.create(document.getElementById('template-editor'), {
                value: '',
                language: 'ttp',
                theme: 'vs-dark',
                fontSize: 13,
                fontFamily: 'Consolas, Monaco, "Courier New", monospace',
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                wordWrap: 'on',
                automaticLayout: true
            });
            
            // Set up error markers listener (global event, not editor method)
            monaco.editor.onDidChangeMarkers((uris) => {
                const model = templateEditor.getModel();
                if (model && uris.includes(model.uri)) {
                    const markers = monaco.editor.getModelMarkers({ resource: model.uri });
                    // Errors will be handled by app.js
                }
            });
            
            // Create output editor (read-only, for displaying results)
            const outputContainer = document.getElementById('output-display');
            // Clear initial message if present
            const initialMsg = outputContainer.querySelector('div');
            if (initialMsg && initialMsg.textContent.includes('Results will appear')) {
                outputContainer.removeChild(initialMsg);
            }
            outputEditor = monaco.editor.create(outputContainer, {
                value: 'Results will appear here after processing...',
                language: 'json',
                theme: 'vs-dark',
                fontSize: 13,
                fontFamily: 'Consolas, Monaco, "Courier New", monospace',
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                wordWrap: 'on',
                automaticLayout: true,
                readOnly: true,
                folding: true,
                lineNumbers: 'on',
                renderWhitespace: 'selection',
                formatOnPaste: false,
                formatOnType: false
            });
            
            resolve();
        });
    });
}

/**
 * Get input editor instance
 */
function getInputEditor() {
    return inputEditor;
}

/**
 * Get template editor instance
 */
function getTemplateEditor() {
    return templateEditor;
}

/**
 * Get output editor instance
 */
function getOutputEditor() {
    return outputEditor;
}

/**
 * Set error markers in template editor
 */
function setTemplateErrors(errors) {
    if (!templateEditor) return;
    
    const model = templateEditor.getModel();
    const markers = errors.map(error => ({
        severity: monaco.MarkerSeverity.Error,
        startLineNumber: error.line || 1,
        startColumn: error.column || 1,
        endLineNumber: error.line || 1,
        endColumn: error.column || 999,
        message: error.message || 'Error'
    }));
    
    monaco.editor.setModelMarkers(model, 'ttp', markers);
}

/**
 * Clear error markers in template editor
 */
function clearTemplateErrors() {
    if (!templateEditor) return;
    
    const model = templateEditor.getModel();
    monaco.editor.setModelMarkers(model, 'ttp', []);
}

/**
 * Set YANG validation error markers in template editor
 * @param {Object} validationResults - Validation results by group name
 */
function setYANGValidationErrors(validationResults) {
    if (!templateEditor) return;
    
    const model = templateEditor.getModel();
    const templateText = model.getValue();
    const markers = [];
    
    // Parse validation results and find corresponding group lines
    for (const [groupName, result] of Object.entries(validationResults)) {
        if (!result || (!result.Errors || result.Errors.length === 0) && (!result.Warnings || result.Warnings.length === 0)) {
            continue;
        }
        
        // Find the group tag in the template
        const groupRegex = new RegExp(`<group[^>]*name=["']${groupName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}["'][^>]*>`, 'i');
        const match = templateText.match(groupRegex);
        if (!match) {
            // Try to find without name attribute (anonymous groups)
            continue;
        }
        
        // Find line number for this group
        const beforeMatch = templateText.substring(0, match.index);
        const lineNumber = (beforeMatch.match(/\n/g) || []).length + 1;
        
        // Add error markers
        if (result.Errors && result.Errors.length > 0) {
            for (const error of result.Errors) {
                markers.push({
                    severity: monaco.MarkerSeverity.Error,
                    startLineNumber: lineNumber,
                    startColumn: 1,
                    endLineNumber: lineNumber,
                    endColumn: 999,
                    message: `YANG Validation Error (${groupName}): ${error.Message || error.message || 'Validation failed'}${error.Field ? ` - Field: ${error.Field}` : ''}`
                });
            }
        }
        
        // Add warning markers
        if (result.Warnings && result.Warnings.length > 0) {
            for (const warning of result.Warnings) {
                markers.push({
                    severity: monaco.MarkerSeverity.Warning,
                    startLineNumber: lineNumber,
                    startColumn: 1,
                    endLineNumber: lineNumber,
                    endColumn: 999,
                    message: `YANG Validation Warning (${groupName}): ${warning.Message || warning.message || 'Validation warning'}${warning.Field ? ` - Field: ${warning.Field}` : ''}`
                });
            }
        }
    }
    
    monaco.editor.setModelMarkers(model, 'yang-validation', markers);
}

// Initialize when DOM is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initMonacoEditor);
} else {
    initMonacoEditor();
}

