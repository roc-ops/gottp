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
 * Get word wrap setting from localStorage
 * @param {string} editorName - Name of the editor ('input', 'template', 'output')
 * @returns {string} - 'on' or 'off'
 */
function getWordWrapSetting(editorName) {
    const stored = localStorage.getItem(`wordWrap_${editorName}`);
    // Default to 'on' if not set
    return stored === 'off' ? 'off' : 'on';
}

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
            
            // Get word wrap settings from localStorage
            const inputWordWrap = getWordWrapSetting('input');
            const templateWordWrap = getWordWrapSetting('template');
            const outputWordWrap = getWordWrapSetting('output');
            
            // Initialize checkboxes with stored values
            document.getElementById('input-word-wrap').checked = inputWordWrap === 'on';
            document.getElementById('template-word-wrap').checked = templateWordWrap === 'on';
            document.getElementById('output-word-wrap').checked = outputWordWrap === 'on';
            
            // Create input editor
            inputEditor = monaco.editor.create(document.getElementById('input-editor'), {
                value: '',
                language: 'plaintext',
                theme: 'vs-dark',
                fontSize: 13,
                fontFamily: 'Consolas, Monaco, "Courier New", monospace',
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                wordWrap: inputWordWrap,
                automaticLayout: true,
                glyphMargin: true, // Enable glyph margin for gutter markers
                lineNumbers: 'on',
                lineDecorationsWidth: 10,
                overviewRulerLanes: 3 // Enable overview ruler for markers
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
                wordWrap: templateWordWrap,
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
                wordWrap: outputWordWrap,
                automaticLayout: true,
                readOnly: true,
                folding: true,
                lineNumbers: 'on',
                renderWhitespace: 'selection',
                formatOnPaste: false,
                formatOnType: false,
                cursorStyle: 'line',
                mouseStyle: 'text'
            });
            
            // Add hover handler for result-to-input navigation
            let hoverTimeout = null;
            // Use global variables for navigation decorations (declared at module level)
            
            outputEditor.onMouseMove((e) => {
                // Only handle if source maps are enabled and we have navigation data
                if (!sourceMapNavigationData.sourceMap || !sourceMapNavigationData.resultPathToMatches || Object.keys(sourceMapNavigationData.resultPathToMatches).length === 0) {
                    return;
                }
                
                if (hoverTimeout) {
                    clearTimeout(hoverTimeout);
                }
                
                // Clear previous hover decorations from input editor
                if (currentInputHoverDecorations.length > 0 && inputEditor) {
                    inputEditor.deltaDecorations(currentInputHoverDecorations, []);
                    currentInputHoverDecorations.length = 0; // Clear array
                }
                
                // Clear previous output decorations
                if (currentOutputHoverDecorations.length > 0) {
                    outputEditor.deltaDecorations(currentOutputHoverDecorations, []);
                    currentOutputHoverDecorations.length = 0; // Clear array
                }
                
                if (e.target && e.target.position) {
                    hoverTimeout = setTimeout(() => {
                        const model = outputEditor.getModel();
                        if (!model) return;
                        
                        const position = e.target.position;
                        
                        // Try to find this word as a result path
                        const resultPath = findResultPathAtPosition(model, position);
                        if (resultPath) {
                            // Highlight the key in output editor
                            const line = model.getLineContent(position.lineNumber);
                            const jsonKeyRegex = new RegExp(`"${resultPath.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}"\\s*:`);
                            const keyMatch = line.match(jsonKeyRegex);
                            if (keyMatch) {
                                const keyStart = keyMatch.index + 1;
                                const keyEnd = keyMatch.index + keyMatch[0].length - 3;
                                // Clear and set output hover decorations
                                currentOutputHoverDecorations.length = 0;
                                const outputHoverDecs = outputEditor.deltaDecorations([], [{
                                    range: new monaco.Range(position.lineNumber, keyStart, position.lineNumber, keyEnd + 1),
                                    options: {
                                        className: 'source-map-hover-highlight',
                                        hoverMessage: { value: `Click to navigate to input (Group: ${resultPath})` }
                                    }
                                }]);
                                currentOutputHoverDecorations.push(...outputHoverDecs);
                            }
                            
                            // Navigate to input (pass position for better match selection)
                            const decorations = navigateResultToInput(resultPath, position);
                            if (decorations && decorations.length > 0) {
                                // Clear and set input hover decorations
                                currentInputHoverDecorations.length = 0;
                                currentInputHoverDecorations.push(...decorations);
                                
                                // Clear decorations after delay
                                setTimeout(() => {
                                    if (currentInputHoverDecorations.length > 0 && inputEditor) {
                                        inputEditor.deltaDecorations(currentInputHoverDecorations, []);
                                        currentInputHoverDecorations.length = 0; // Clear array
                                    }
                                    if (currentOutputHoverDecorations.length > 0) {
                                        outputEditor.deltaDecorations(currentOutputHoverDecorations, []);
                                        currentOutputHoverDecorations.length = 0; // Clear array
                                    }
                                }, 2000);
                            }
                        }
                    }, 300); // Delay to avoid flickering
                }
            });
            
            // Add click handler for result-to-input navigation
            outputEditor.onMouseDown((e) => {
                // Only handle if source maps are enabled and we have navigation data
                if (!sourceMapNavigationData.sourceMap || !sourceMapNavigationData.resultPathToMatches || Object.keys(sourceMapNavigationData.resultPathToMatches).length === 0) {
                    return;
                }
                
                if (e.target && e.target.position) {
                    const model = outputEditor.getModel();
                    if (!model) return;
                    
                    const position = e.target.position;
                    const resultPath = findResultPathAtPosition(model, position);
                    
                    if (resultPath) {
                        // Clear any existing hover decorations
                        if (currentInputHoverDecorations.length > 0 && inputEditor) {
                            inputEditor.deltaDecorations(currentInputHoverDecorations, []);
                            currentInputHoverDecorations.length = 0; // Clear array
                        }
                        if (currentOutputHoverDecorations.length > 0) {
                            outputEditor.deltaDecorations(currentOutputHoverDecorations, []);
                            currentOutputHoverDecorations.length = 0; // Clear array
                        }
                        
                        // Clear previous click decorations before creating new ones
                        if (currentInputClickDecorations.length > 0 && inputEditor) {
                            inputEditor.deltaDecorations(currentInputClickDecorations, []);
                            currentInputClickDecorations.length = 0; // Clear array
                        }
                        
                        // Pass the output position to help select the right match
                        const clickDecorations = navigateResultToInput(resultPath, position);
                        if (clickDecorations && clickDecorations.length > 0) {
                            // Clear and set click decorations
                            currentInputClickDecorations.length = 0;
                            currentInputClickDecorations.push(...clickDecorations);
                        }
                    }
                }
            });
            
            // Add click handler for input-to-result navigation
            inputEditor.onMouseDown((e) => {
                // Only handle if source maps are enabled and we have navigation data
                if (!sourceMapNavigationData.sourceMap || !sourceMapNavigationData.resultPathToMatches || Object.keys(sourceMapNavigationData.resultPathToMatches).length === 0) {
                    return;
                }
                
                if (e.target && e.target.position) {
                    const model = inputEditor.getModel();
                    if (!model) return;
                    
                    const position = e.target.position;
                    
                    // Find which match this position corresponds to
                    const inputSourceMap = sourceMapNavigationData.sourceMap?.Inputs?.['Default_Input'];
                    if (!inputSourceMap || !inputSourceMap.Lines) {
                        return;
                    }
                    
                    const lineIndex = position.lineNumber - 1;
                    if (lineIndex >= 0 && lineIndex < inputSourceMap.Lines.length) {
                        const lineMapping = inputSourceMap.Lines[lineIndex];
                        if (lineMapping.Matches && lineMapping.Matches.length > 0) {
                            // Find the match that contains this column position
                            const column = position.column - 1; // Convert to 0-indexed
                            for (let matchIdx = 0; matchIdx < lineMapping.Matches.length; matchIdx++) {
                                const match = lineMapping.Matches[matchIdx];
                                if (column >= match.StartCol && column < match.EndCol) {
                                    // First, check if the click is on a variable within this match
                                    let clickedVariable = null;
                                    if (match.Variables) {
                                        for (const [varName, varRange] of Object.entries(match.Variables)) {
                                            // Check if column is within variable range
                                            // Variables use 0-indexed columns (StartCol inclusive, EndCol exclusive)
                                            if (column >= varRange.StartCol && column < varRange.EndCol) {
                                                clickedVariable = varName;
                                                break;
                                            }
                                        }
                                    }
                                    
                                    const resultPath = match.ResultPath || match.GroupName || '';
                                    if (resultPath) {
                                        // Find the match order for this specific match
                                        // We need to find the match that corresponds to this exact input position
                                        const matches = sourceMapNavigationData.resultPathToMatches[resultPath];
                                        let matchOrder = null;
                                        if (matches) {
                                            // Find which match in the array corresponds to this input line/match
                                            // Match by line number and start column to get the exact match
                                            for (let i = 0; i < matches.length; i++) {
                                                if (matches[i].lineNumber === lineIndex + 1 && 
                                                    matches[i].startCol === match.StartCol) {
                                                    matchOrder = matches[i].matchOrder;
                                                    break;
                                                }
                                            }
                                        }
                                        
                                        // If we clicked on a variable, navigate to that specific property
                                        if (clickedVariable) {
                                            // Construct path to the variable: resultPath.variableName
                                            // For array paths, we need to navigate to the specific array item first
                                            const variablePath = resultPath + '.' + clickedVariable;
                                            navigateToResultPath(variablePath, matchOrder);
                                        } else {
                                            // Navigate to the object/array itself
                                            navigateToResultPath(resultPath, matchOrder);
                                        }
                                    }
                                    break;
                                }
                            }
                        }
                    }
                }
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

// Global state for source map navigation
let sourceMapNavigationData = {
    sourceMap: null,
    resultPathToMatches: {}, // result path -> array of match info
    matchToResultPath: {}, // match key -> result path
    groupNameToResultPath: {}, // group name -> result path (for fallback)
    lineToMatchOrder: null // Map from line index to match order for array indexing
};

// Global state for navigation decorations (exposed to window for cleanup)
let currentInputClickDecorations = [];
let currentInputHoverDecorations = [];
let currentOutputHoverDecorations = [];

// Expose to window for cleanup from app.js
if (typeof window !== 'undefined') {
    window.currentInputClickDecorations = currentInputClickDecorations;
    window.currentInputHoverDecorations = currentInputHoverDecorations;
    window.currentOutputHoverDecorations = currentOutputHoverDecorations;
}

/**
 * Build navigation data structure from source map
 * @param {Object} sourceMap - Source map data
 * @param {string} inputName - Input name
 */
function buildSourceMapNavigationData(sourceMap, inputName = 'Default_Input') {
    sourceMapNavigationData.sourceMap = sourceMap;
    sourceMapNavigationData.resultPathToMatches = {};
    sourceMapNavigationData.matchToResultPath = {};
    sourceMapNavigationData.groupNameToResultPath = {};
    sourceMapNavigationData.lineToMatchOrder = null;
    
    if (!sourceMap || !sourceMap.Inputs || !sourceMap.Inputs[inputName]) {
        return;
    }
    
    const inputSourceMap = sourceMap.Inputs[inputName];
    if (!inputSourceMap || !inputSourceMap.Lines) {
        return;
    }
    
    // Build mapping from result paths to input positions
    // Track match order per result path to map to specific array items
    const matchOrderPerPath = {}; // resultPath -> counter
    
    // First pass: collect all matches for each result path to understand the order
    const matchesByPath = {}; // resultPath -> array of {lineIndex, matchIndex, match}
    inputSourceMap.Lines.forEach((lineMapping, lineIndex) => {
        if (lineMapping.Matches && lineMapping.Matches.length > 0) {
            lineMapping.Matches.forEach((match, matchIndex) => {
                const resultPath = match.ResultPath || match.GroupName || '';
                if (resultPath) {
                    if (!matchesByPath[resultPath]) {
                        matchesByPath[resultPath] = [];
                    }
                    matchesByPath[resultPath].push({lineIndex, matchIndex, match});
                }
            });
        }
    });
    
    // Second pass: group matches by array object and assign matchOrder
    // Matches from the same array object should have the same matchOrder
    // We group matches by looking for gaps in line numbers (gaps indicate new objects)
    const objectGroupsByPath = {}; // resultPath -> array of object groups (each group is array of match indices)
    
    // Group matches by object: consecutive matches with same resultPath belong to same object
    Object.keys(matchesByPath).forEach(resultPath => {
        const matches = matchesByPath[resultPath];
        if (matches.length === 0) return;
        
        const groups = [];
        let currentGroup = [0]; // Start with first match
        
        for (let i = 1; i < matches.length; i++) {
            const prevLine = matches[i - 1].lineIndex;
            const currLine = matches[i].lineIndex;
            const lineGap = currLine - prevLine;
            
            // If there's a gap of more than 1 line, it's likely a new object
            // Also, if line numbers are decreasing, it's a new object
            if (lineGap > 1 || lineGap < 0) {
                groups.push(currentGroup);
                currentGroup = [i];
            } else {
                currentGroup.push(i);
            }
        }
        groups.push(currentGroup); // Add last group
        objectGroupsByPath[resultPath] = groups;
    });
    
    // Third pass: assign matchOrder based on object group
    inputSourceMap.Lines.forEach((lineMapping, lineIndex) => {
        if (lineMapping.Matches && lineMapping.Matches.length > 0) {
            lineMapping.Matches.forEach((match, matchIndex) => {
                const resultPath = match.ResultPath || match.GroupName || '';
                const groupName = match.GroupName || '';
                const matchKey = `${lineIndex}-${matchIndex}`;
                
                // Create a unique match identifier for this specific match
                const matchId = `${lineIndex}-${matchIndex}`;
                
                // Store group name to result path mapping for fallback
                if (groupName && resultPath) {
                    sourceMapNavigationData.groupNameToResultPath[groupName] = resultPath;
                }
                
                // Get match order: find which object group this match belongs to
                let matchOrder = null;
                if (resultPath && matchesByPath[resultPath] && objectGroupsByPath[resultPath]) {
                    // Find the match index in matchesByPath
                    const matchIndexInPath = matchesByPath[resultPath].findIndex(
                        m => m.lineIndex === lineIndex && m.matchIndex === matchIndex
                    );
                    
                    if (matchIndexInPath >= 0) {
                        // Find which object group contains this match index
                        const groups = objectGroupsByPath[resultPath];
                        for (let groupIdx = 0; groupIdx < groups.length; groupIdx++) {
                            if (groups[groupIdx].includes(matchIndexInPath)) {
                                matchOrder = groupIdx; // matchOrder is the object index
                                break;
                            }
                        }
                    }
                }
                
                // If matchOrder is still null, use sequential counter (fallback)
                if (matchOrder === null && resultPath) {
                    if (!matchOrderPerPath[resultPath]) {
                        matchOrderPerPath[resultPath] = 0;
                    }
                    matchOrder = matchOrderPerPath[resultPath]++;
                }
                
                // Create match info with unique identifier
                const matchInfo = {
                    lineNumber: lineIndex + 1,
                    startCol: match.StartCol,
                    endCol: match.EndCol,
                    groupName: match.GroupName,
                    patternIndex: match.PatternIndex,
                    matchId: matchId, // Unique identifier for this match
                    matchOrder: matchOrder // Order within this result path (for array indexing)
                };
                
                if (resultPath) {
                    if (!sourceMapNavigationData.resultPathToMatches[resultPath]) {
                        sourceMapNavigationData.resultPathToMatches[resultPath] = [];
                    }
                    sourceMapNavigationData.resultPathToMatches[resultPath].push(matchInfo);
                    
                    // Also map individual variable names to their full path (resultPath.variableName)
                    // This allows clicking on properties in the output to navigate back to the input
                    if (match.Variables) {
                        Object.keys(match.Variables).forEach(varName => {
                            const variablePath = resultPath + '.' + varName;
                            if (!sourceMapNavigationData.resultPathToMatches[variablePath]) {
                                sourceMapNavigationData.resultPathToMatches[variablePath] = [];
                            }
                            // Use the same matchInfo so variables point to the same input location
                            sourceMapNavigationData.resultPathToMatches[variablePath].push(matchInfo);
                        });
                    }
                }
                
                // Also map by group name as fallback
                if (groupName && groupName !== resultPath) {
                    if (!sourceMapNavigationData.resultPathToMatches[groupName]) {
                        sourceMapNavigationData.resultPathToMatches[groupName] = [];
                    }
                    sourceMapNavigationData.resultPathToMatches[groupName].push(matchInfo);
                }
                
                sourceMapNavigationData.matchToResultPath[matchKey] = resultPath;
            });
        }
    });
}

/**
 * Navigate from input to result (click handler)
 * @param {number} lineNumber - Line number in input
 * @param {number} column - Column number in input
 */
function navigateInputToResult(lineNumber, column) {
    if (!sourceMapNavigationData.sourceMap) {
        return;
    }
    
    const inputName = 'Default_Input';
    const inputSourceMap = sourceMapNavigationData.sourceMap.Inputs[inputName];
    if (!inputSourceMap || !inputSourceMap.Lines) {
        return;
    }
    
    const lineIndex = lineNumber - 1;
    if (lineIndex < 0 || lineIndex >= inputSourceMap.Lines.length) {
        return;
    }
    
    const lineMapping = inputSourceMap.Lines[lineIndex];
    if (!lineMapping.Matches || lineMapping.Matches.length === 0) {
        return;
    }
    
    // Find the match that contains this column
    let targetMatch = null;
    for (const match of lineMapping.Matches) {
        // Check if column is within match range
        if (column >= match.StartCol + 1 && column <= match.EndCol + 1) {
            targetMatch = match;
            break;
        }
    }
    
    // If no specific match found, use the first match on the line
    if (!targetMatch && lineMapping.Matches.length > 0) {
        targetMatch = lineMapping.Matches[0];
    }
    
    if (targetMatch) {
        const resultPath = targetMatch.ResultPath || targetMatch.GroupName || '';
        if (resultPath) {
            navigateToResultPath(resultPath);
        }
    }
}

/**
 * Navigate to a result path in the output editor
 * @param {string} resultPath - Path in result structure (e.g., "interfaces[0]")
 */
function navigateToResultPath(resultPath, matchOrder = null) {
    const outputEditor = getOutputEditor();
    if (!outputEditor || !resultPath) {
        return;
    }
    
    const model = outputEditor.getModel();
    if (!model) {
        return;
    }
    
    const content = model.getValue();
    const lines = content.split('\n');
    let targetLine = -1;
    let targetColumn = 1;
    
    // Remove formatters (* and **) from path for matching
    const pathWithoutFormatters = resultPath.replace(/\*+$/, '');
    
    // Check if the path contains a variable (e.g., "cms:show-cable-modem.modem-entry*.mac-domain")
    const pathParts = pathWithoutFormatters.split('.');
    const hasVariable = pathParts.length > 1;
    const variableName = hasVariable ? pathParts[pathParts.length - 1] : null;
    const basePath = hasVariable ? pathParts.slice(0, -1).join('.') : pathWithoutFormatters;
    
    // Strategy 1: Try exact match first using basePath (without variable if present)
    // This ensures we find the array key, not the property
    // Remove asterisks from basePath for matching (they're formatters, not part of the key name)
    const searchPath = basePath.replace(/\*+$/, ''); // Remove trailing asterisks
    const escapedPath = searchPath.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    let arrayKeyLine = -1;
    
    for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        const jsonKeyMatch = line.match(new RegExp(`"${escapedPath}"\\s*:`));
        if (jsonKeyMatch) {
            arrayKeyLine = i + 1;
            targetLine = i + 1;
            targetColumn = jsonKeyMatch.index + 1;
            break;
        }
    }
    
    // Strategy 2: Split path by dots only (colons might be part of the key name)
    // Handle paths like "cms:show-cable-modem.modem-entry" where the first part has a colon
    if (arrayKeyLine <= 0) {
        const dotParts = basePath.replace(/\*+$/, '').split('.').filter(p => p.length > 0);
        
        if (dotParts.length > 0) {
            // Try the last part of basePath (most specific, excluding variable)
            const lastPart = dotParts[dotParts.length - 1];
            const lastPartEscaped = lastPart.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
            
            for (let i = 0; i < lines.length; i++) {
                const line = lines[i];
                const jsonKeyMatch = line.match(new RegExp(`"${lastPartEscaped}"\\s*:`));
                if (jsonKeyMatch) {
                    arrayKeyLine = i + 1;
                    targetLine = i + 1;
                    targetColumn = jsonKeyMatch.index + 1;
                    break;
                }
            }
        }
    }
    
    // Strategy 3: Try first part (might contain colons)
    if (arrayKeyLine <= 0 && basePath.includes('.')) {
        const firstPart = basePath.replace(/\*+$/, '').split('.')[0];
        const firstPartEscaped = firstPart.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        
        for (let i = 0; i < lines.length; i++) {
            const line = lines[i];
            const indentMatch = line.match(/^(\s*)/);
            const indent = indentMatch ? indentMatch[1].length : 0;
            
            // Top-level keys typically have 0-4 spaces of indentation
            if (indent <= 4) {
                const jsonKeyMatch = line.match(new RegExp(`"${firstPartEscaped}"\\s*:`));
                if (jsonKeyMatch) {
                    arrayKeyLine = i + 1;
                    targetLine = i + 1;
                    targetColumn = jsonKeyMatch.index + 1;
                    break;
                }
            }
        }
    }
    
    // If we found the array key and have a match order, navigate to the specific array item
    if (arrayKeyLine > 0 && matchOrder !== null && matchOrder !== undefined) {
        // First, count how many objects are actually in the array
        let arrayStartLine = -1;
        let totalObjects = 0;
        const arrayKeyIndent = lines[arrayKeyLine - 1].match(/^(\s*)/)?.[1]?.length || 0;
        let expectedIndent = arrayKeyIndent + 2;
        
        // Find array start
        for (let i = arrayKeyLine - 1; i < lines.length && i < arrayKeyLine + 10; i++) {
            const line = lines[i];
            if (line.includes('[')) {
                arrayStartLine = i + 1;
                // Determine actual indent by looking at first object
                for (let j = arrayStartLine; j < lines.length && j < arrayStartLine + 5; j++) {
                    const checkLine = lines[j];
                    const checkTrimmed = checkLine.trim();
                    if (checkTrimmed.startsWith('{')) {
                        expectedIndent = checkLine.match(/^(\s*)/)?.[1]?.length || 0;
                        break;
                    }
                }
                break;
            }
        }
        
        // Count total objects in array (scan until closing bracket)
        if (arrayStartLine > 0) {
            let braceDepth = 0;
            let inArray = false;
            for (let i = arrayStartLine - 1; i < lines.length; i++) {
                const line = lines[i];
                const trimmed = line.trim();
                const indent = line.match(/^(\s*)/)?.[1]?.length || 0;
                
                if (!inArray && trimmed.includes('[')) {
                    inArray = true;
                    braceDepth = 1;
                    continue;
                }
                
                if (inArray) {
                    // Count opening braces for objects at array level
                    if (trimmed.startsWith('{') && indent === expectedIndent) {
                        totalObjects++;
                    }
                    
                    // Track brace depth
                    for (const char of line) {
                        if (char === '[') braceDepth++;
                        if (char === ']') braceDepth--;
                        if (char === '{') braceDepth++;
                        if (char === '}') braceDepth--;
                    }
                    
                    // End of array
                    if (braceDepth === 0) {
                        break;
                    }
                }
            }
        }
        
        // If matchOrder is out of range or array is empty, don't proceed
        if (totalObjects === 0) {
            // Fallback: just navigate to the array key line
            targetLine = arrayKeyLine;
            targetColumn = 1;
        } else {
            let targetMatchOrder = matchOrder;
            if (matchOrder >= totalObjects) {
                targetMatchOrder = Math.max(0, totalObjects - 1);
            }
            
            // Now find the specific object
            let objectCount = 0;
            let braceDepth = 0;
            let inArray = false;
            
            for (let i = arrayStartLine - 1; i < lines.length; i++) {
                if (i < 0 || i >= lines.length) break;
                const line = lines[i];
                if (!line) break;
                const trimmed = line.trim();
                const indent = line.match(/^(\s*)/)?.[1]?.length || 0;
                
                if (!inArray && trimmed.includes('[')) {
                    inArray = true;
                    braceDepth = 1;
                    continue;
                }
                
                if (inArray) {
                    // Check if this line starts a new object at array level
                    if (trimmed.startsWith('{') && indent === expectedIndent) {
                        if (objectCount === targetMatchOrder) {
                            // Found the target object
                            const objectStartLine = i + 1;
                            
                            // If we have a variable name, find that property within this object
                            if (variableName) {
                                const variableEscaped = variableName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
                                const objectIndent = indent;
                                const propertyIndent = objectIndent + 2; // Properties are indented 2 spaces inside object
                                
                                // Search within this object for the variable property
                                let objectBraceDepth = 1;
                                let foundProperty = false;
                                
                                for (let j = i; j < lines.length && objectBraceDepth > 0; j++) {
                                    const propLine = lines[j];
                                    const propTrimmed = propLine.trim();
                                    const propIndent = propLine.match(/^(\s*)/)?.[1]?.length || 0;
                                    
                                    // Check if this line contains the property we're looking for
                                    if (propIndent === propertyIndent) {
                                        const propMatch = propLine.match(new RegExp(`"${variableEscaped}"\\s*:`));
                                    if (propMatch) {
                                        targetLine = j + 1;
                                        targetColumn = propMatch.index + 1;
                                        foundProperty = true;
                                        break;
                                    }
                                    }
                                    
                                    // Track object brace depth
                                    for (const char of propLine) {
                                        if (char === '{') objectBraceDepth++;
                                        if (char === '}') objectBraceDepth--;
                                    }
                                }
                                
                            if (!foundProperty) {
                                // Fallback to object start if property not found
                                targetLine = objectStartLine;
                                targetColumn = line.indexOf('{') + 1;
                            }
                        } else {
                            // No variable, just navigate to object start
                            targetLine = i + 1;
                            targetColumn = line.indexOf('{') + 1;
                        }
                            break;
                        }
                        objectCount++;
                    }
                    
                    // Track brace depth to know when array ends
                    for (const char of line) {
                        if (char === '[') braceDepth++;
                        if (char === ']') braceDepth--;
                        if (char === '{') braceDepth++;
                        if (char === '}') braceDepth--;
                    }
                    
                    if (braceDepth === 0) {
                        // End of array
                        break;
                    }
                }
            }
            
            if (targetLine <= 0) {
                // Fallback to array key line
                targetLine = arrayKeyLine;
                targetColumn = 1;
            }
        }
    }
    
    if (targetLine > 0) {
        // Navigate to the line and column
        outputEditor.setPosition({ lineNumber: targetLine, column: targetColumn });
        outputEditor.revealLineInCenter(targetLine);
        
        // If we have a variable name, highlight just the property key
        let highlightRange;
        if (variableName && targetLine > 0) {
            // Find the property key on the target line
            const targetLineContent = lines[targetLine - 1];
            const variableEscaped = variableName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
            const propMatch = targetLineContent.match(new RegExp(`"${variableEscaped}"\\s*:`));
            if (propMatch) {
                // Highlight from opening quote to closing quote
                const keyStart = propMatch.index + 1;
                const matchStr = propMatch[0];
                const closingQuoteIndex = matchStr.indexOf('"', 1);
                const keyEnd = propMatch.index + closingQuoteIndex + 2; // Include closing quote
                highlightRange = new monaco.Range(targetLine, keyStart, targetLine, keyEnd);
            } else {
                // Fallback to whole line
                highlightRange = new monaco.Range(targetLine, 1, targetLine, 1000);
            }
        } else {
            // Highlight the whole line for object/array navigation
            highlightRange = new monaco.Range(targetLine, 1, targetLine, 1000);
        }
        
        // Highlight briefly
        const decorations = outputEditor.deltaDecorations([], [{
            range: highlightRange,
            options: {
                className: 'source-map-hover-highlight',
                isWholeLine: !variableName,
                hoverMessage: { value: `Result path: ${resultPath}` }
            }
        }]);
        
        // Remove highlight after 2 seconds
        setTimeout(() => {
            outputEditor.deltaDecorations(decorations, []);
        }, 2000);
    }
}

/**
 * Navigate from result to input (hover/click handler)
 * @param {string} resultPath - Path in result structure
 * @param {Object} outputPosition - Optional output editor position to help select the right match
 */
function navigateResultToInput(resultPath, outputPosition = null) {
    // Try exact match first
    let matches = sourceMapNavigationData.resultPathToMatches[resultPath];
    
    // If no exact match, try group name fallback
    if (!matches || matches.length === 0) {
        // Try to find by group name
        const groupName = resultPath.split('.')[0]; // Get first part of path
        matches = sourceMapNavigationData.resultPathToMatches[groupName];
    }
    
    // If still no match, try all keys that contain this path
    if (!matches || matches.length === 0) {
        for (const [key, value] of Object.entries(sourceMapNavigationData.resultPathToMatches)) {
            if (key.includes(resultPath) || resultPath.includes(key)) {
                matches = value;
                break;
            }
        }
    }
    
    if (!matches || matches.length === 0) {
        return [];
    }
    
    // If we have multiple matches and an output position, try to find the best match
    // by looking at the output JSON structure to determine which array item we're in
    let match = matches[0];
    if (matches.length > 1 && outputPosition) {
        const outputEditor = getOutputEditor();
        const outputModel = outputEditor?.getModel();
        
        if (outputModel) {
            // Find which array item this output position is in
            const outputLine = outputPosition.lineNumber;
            const outputContent = outputModel.getValue();
            const outputLines = outputContent.split('\n');
            
            // Look backwards from the clicked line to find the array item start
            let arrayItemIndex = -1;
            let arrayStartLine = -1;
            let arrayKeyIndent = -1;
            let expectedIndent = -1;
            
            // Strategy: Find which object contains the clicked line, then count objects from array start
            // First, find the object start by looking backwards for the opening brace
            let objectStartLine = -1;
            let objectIndent = -1;
            
            // Look backwards to find the opening brace of the object containing this line
            // We need to track brace depth to find the object that contains the clicked line
            let braceDepth = 0;
            let bracketDepth = 0;
            
            // First, scan backwards to find where we are in the structure
            for (let i = outputLine - 1; i >= 0 && i >= outputLine - 1000; i--) {
                const line = outputLines[i];
                const trimmed = line.trim();
                const indent = line.match(/^(\s*)/)?.[1]?.length || 0;
                
                // Track depth by counting braces and brackets
                for (let j = line.length - 1; j >= 0; j--) {
                    const char = line[j];
                    if (char === '}') braceDepth++;
                    if (char === '{') braceDepth--;
                    if (char === ']') bracketDepth++;
                    if (char === '[') bracketDepth--;
                }
                
                // If we find an opening brace at the right indent level and we're in an array, this might be our object
                if (trimmed.startsWith('{') && bracketDepth > 0) {
                    // Check if this brace is at array-item level (indent should be consistent)
                    // We want the brace that starts the object containing our clicked line
                    if (braceDepth === 0 && bracketDepth > 0) {
                        objectStartLine = i + 1;
                        objectIndent = indent;
                        break;
                    }
                }
            }
            
            // Alternative: if we didn't find it with depth tracking, look for the nearest opening brace
            if (objectStartLine === -1) {
                // Look backwards for opening brace that's likely an object start
                for (let i = outputLine - 1; i >= 0 && i >= outputLine - 500; i--) {
                    const line = outputLines[i];
                    const trimmed = line.trim();
                    const indent = line.match(/^(\s*)/)?.[1]?.length || 0;
                    
                    // Look for opening brace that's indented (not at start of line, likely an object)
                    if (trimmed.startsWith('{') && indent > 0) {
                        // Check if next few lines look like object properties
                        let looksLikeObject = false;
                        for (let j = i + 1; j < Math.min(i + 5, outputLines.length); j++) {
                            const nextLine = outputLines[j];
                            if (nextLine.includes(':') || nextLine.includes('"')) {
                                looksLikeObject = true;
                                break;
                            }
                        }
                        
                        if (looksLikeObject) {
                            objectStartLine = i + 1;
                            objectIndent = indent;
                            break;
                        }
                    }
                }
            }
            
            // Now find the array key - look for "cms:show-cable-modem" or similar
            // Get the group name from matches to find the right array key
            let arrayKeyName = null;
            if (matches.length > 0 && matches[0].groupName) {
                // Try to find the result path that contains this group
                for (const [key, value] of Object.entries(sourceMapNavigationData.resultPathToMatches)) {
                    if (key.includes('modem-entry') || key.includes('cms')) {
                        const pathParts = key.split('.');
                        arrayKeyName = pathParts[0]; // e.g., "cms:show-cable-modem"
                        break;
                    }
                }
            }
            
            // If we found the object start, now find the array that contains it
            if (objectStartLine > 0 && objectIndent >= 0) {
                // Look backwards from object start to find the array key
                // The array key should be before the object, with less indentation
                for (let i = objectStartLine - 1; i >= 0 && i >= objectStartLine - 100; i--) {
                    const line = outputLines[i];
                    const trimmed = line.trim();
                    const indent = line.match(/^(\s*)/)?.[1]?.length || 0;
                    
                    // Look for array key pattern: "key": [
                    if (trimmed.includes('[') && indent < objectIndent) {
                        // Check if this looks like an array key (has a key name before the bracket)
                        const keyMatch = line.match(/"([^"]+)"\s*:\s*\[/);
                        if (keyMatch) {
                            arrayStartLine = i + 1;
                            arrayKeyIndent = indent;
                            expectedIndent = objectIndent;
                            
                            break;
                        } else if (trimmed.startsWith('[')) {
                            // Just an opening bracket, might be the array start
                            arrayStartLine = i + 1;
                            arrayKeyIndent = indent;
                            expectedIndent = objectIndent;
                            break;
                        }
                    }
                }
            }
            
            // Fallback: search from beginning of file for array key
            if (arrayStartLine === -1) {
                // Try common array key names
                const possibleKeys = ['cms:show-cable-modem', 'modem-entry'];
                
                for (const keyName of possibleKeys) {
                    const escapedKeyName = keyName.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
                    for (let i = 0; i < Math.min(50, outputLines.length); i++) {
                        const line = outputLines[i];
                        const keyMatch = line.match(new RegExp(`"${escapedKeyName}"\\s*:\\s*\\[`));
                        if (keyMatch) {
                            arrayStartLine = i + 1;
                            arrayKeyIndent = line.match(/^(\s*)/)?.[1]?.length || 0;
                            
                            // Determine actual indent by looking at first object
                            for (let j = arrayStartLine; j < outputLines.length && j < arrayStartLine + 10; j++) {
                                const objLine = outputLines[j];
                                const objTrimmed = objLine.trim();
                                if (objTrimmed.startsWith('{')) {
                                    expectedIndent = objLine.match(/^(\s*)/)?.[1]?.length || 0;
                                    break;
                                }
                            }
                            
                            break;
                        }
                    }
                    if (arrayStartLine > 0) break;
                }
            }
            
            // If we still don't have array start but have object start, use object indent to infer array
            if (arrayStartLine === -1 && objectStartLine > 0 && objectIndent >= 0) {
                // Look backwards from object for opening bracket
                for (let i = objectStartLine - 1; i >= 0 && i >= objectStartLine - 200; i--) {
                    const line = outputLines[i];
                    if (line.includes('[')) {
                        arrayStartLine = i + 1;
                        arrayKeyIndent = line.match(/^(\s*)/)?.[1]?.length || 0;
                        expectedIndent = objectIndent;
                        break;
                    }
                }
            }
            
            // Strategy 2: If we didn't find the key, look backwards for opening bracket
            // Track brace depth to find the array we're in
            if (arrayStartLine === -1) {
                let braceDepth = 0;
                let bracketDepth = 0;
                let foundArrayStart = false;
                
                // Look backwards to find the array start
                for (let i = outputLine - 1; i >= 0 && i >= outputLine - 10000; i--) {
                    const line = outputLines[i];
                    const trimmed = line.trim();
                    
                    // Track depth
                    for (const char of line) {
                        if (char === '}') braceDepth++;
                        if (char === '{') braceDepth--;
                        if (char === ']') bracketDepth++;
                        if (char === '[') {
                            bracketDepth--;
                            // If we find an opening bracket and we're at the right depth, this might be our array
                            if (bracketDepth === 0 && braceDepth === 0) {
                                const indent = line.match(/^(\s*)/)?.[1]?.length || 0;
                                if (indent <= 10) { // Reasonable top-level array
                                    arrayStartLine = i + 1;
                                    arrayKeyIndent = indent;
                                    
                                    // Determine actual indent by looking at first object
                                    for (let j = arrayStartLine; j < outputLines.length && j < arrayStartLine + 10; j++) {
                                        const checkLine = outputLines[j];
                                        const checkTrimmed = checkLine.trim();
                                        if (checkTrimmed.startsWith('{')) {
                                            expectedIndent = checkLine.match(/^(\s*)/)?.[1]?.length || 0;
                                            break;
                                        }
                                    }
                                    
                                    foundArrayStart = true;
                                    break;
                                }
                            }
                        }
                    }
                    
                    if (foundArrayStart) break;
                }
            }
            
            // Now count objects from the array start to the object containing the clicked line
            if (arrayStartLine > 0 && expectedIndent >= 0 && objectStartLine > 0) {
                arrayItemIndex = -1; // Start at -1, will increment to 0 for first object
                
                // Count objects from array start up to and including the object containing the clicked line
                for (let j = arrayStartLine; j <= objectStartLine; j++) {
                    const checkLine = outputLines[j];
                    const checkTrimmed = checkLine.trim();
                    const checkIndent = checkLine.match(/^(\s*)/)?.[1]?.length || 0;
                    
                    // Check if this line starts a new object at array level
                    if (checkTrimmed.startsWith('{') && checkIndent === expectedIndent) {
                        arrayItemIndex++;
                        // If this is the object containing our clicked line, stop counting
                        if (j === objectStartLine) {
                            break;
                        }
                    }
                }
                
            } else {
            }
            
            // If we found an array item index, find the match with that order
            if (arrayStartLine > 0 && arrayItemIndex >= 0) {
                // Use the array item index directly as match order
                const targetMatchOrder = arrayItemIndex;
                
                
                // Find match with matching order
                const foundMatch = matches.find(m => m.matchOrder === targetMatchOrder);
                if (foundMatch) {
                    match = foundMatch;
                } else if (targetMatchOrder < matches.length) {
                    // Fallback: use match at target match order
                    match = matches[targetMatchOrder];
                } else {
                }
            } else {
            }
        }
    }
    
    const inputEditor = getInputEditor();
    if (!inputEditor) {
        return [];
    }
    
    // Navigate to the line (adjust for off-by-one if needed)
    // The match.lineNumber is 1-indexed, so it should be correct
    // But if user reports it's one line too much, we might need to adjust
    const targetLine = match.lineNumber;
    
    // Check if resultPath contains a variable name (e.g., "version-info*.file-name")
    // If so, we should highlight just that variable, not the whole match
    const pathParts = resultPath.split('.');
    const hasVariable = pathParts.length > 1;
    const variableName = hasVariable ? pathParts[pathParts.length - 1] : null;
    
    let highlightRange;
    if (hasVariable && variableName) {
        // Find the variable in the source map match
        const inputSourceMap = sourceMapNavigationData.sourceMap?.Inputs?.['Default_Input'];
        if (inputSourceMap && inputSourceMap.Lines) {
            const lineMapping = inputSourceMap.Lines[targetLine - 1];
            if (lineMapping && lineMapping.Matches) {
                // Find the match that corresponds to this match info
                const sourceMatch = lineMapping.Matches.find(m => 
                    m.StartCol === match.startCol && 
                    (m.ResultPath || m.GroupName) === (match.groupName || resultPath.split('.')[0])
                );
                
                if (sourceMatch && sourceMatch.Variables && sourceMatch.Variables[variableName]) {
                    // Highlight just the variable range
                    const varRange = sourceMatch.Variables[variableName];
                    highlightRange = new monaco.Range(
                        targetLine, 
                        varRange.StartCol + 1, 
                        targetLine, 
                        varRange.EndCol + 1
                    );
                }
            }
        }
    }
    
    // If we didn't find a variable range, highlight the whole match
    if (!highlightRange) {
        highlightRange = new monaco.Range(match.lineNumber, match.startCol + 1, match.lineNumber, match.endCol + 1);
    }
    
    inputEditor.setPosition({ lineNumber: targetLine, column: highlightRange.startColumn });
    inputEditor.revealLineInCenter(targetLine);
    
    // Highlight the match range or variable range
    const decorations = inputEditor.deltaDecorations([], [{
        range: highlightRange,
        options: {
            className: 'source-map-hover-highlight',
            hoverMessage: { value: `Group: ${match.groupName || 'unnamed'}, Pattern: ${match.patternIndex}${variableName ? `, Variable: ${variableName}` : ''}` }
        }
    }]);
    
    // Store decoration ID for cleanup
    return decorations;
}

/**
 * Apply source map decorations to input editor
 * @param {Object} sourceMap - Source map data
 * @param {string} inputName - Input name (default: 'Default_Input')
 * @returns {string[]} Decoration IDs for later removal
 */
function applySourceMapDecorations(sourceMap, inputName = 'Default_Input') {
    if (!inputEditor || !sourceMap || !sourceMap.Inputs) {
        return [];
    }
    
    const inputSourceMap = sourceMap.Inputs[inputName];
    if (!inputSourceMap || !inputSourceMap.Lines) {
        return [];
    }
    
    const model = inputEditor.getModel();
    const decorations = [];
    
    const maxLineCount = model.getLineCount();
    
    // Track which line numbers we've already decorated to avoid duplicates
    const decoratedLines = new Set();
    
    // Apply decorations for each line
    // Use lineMapping.LineNumber (0-indexed) instead of array index
    inputSourceMap.Lines.forEach((lineMapping, lineIndex) => {
        // Skip if lineMapping is invalid or empty
        if (!lineMapping) {
            return;
        }
        
        // Use the LineNumber from the source map (0-indexed) and convert to 1-based for Monaco
        const lineNumber = (lineMapping.LineNumber !== undefined && lineMapping.LineNumber !== null) 
            ? lineMapping.LineNumber + 1 
            : lineIndex + 1; // Fallback to array index if LineNumber not set
        
        // Check if line exists - skip if beyond actual line count or invalid
        if (lineNumber < 1 || lineNumber > maxLineCount) {
            return; // Skip non-existent or invalid lines
        }
        
        // Skip if we've already decorated this line number (avoid duplicates)
        if (decoratedLines.has(lineNumber)) {
            return;
        }
        
        // Mark this line as decorated
        decoratedLines.add(lineNumber);
        
        const line = model.getLineContent(lineNumber);
        const lineLength = line ? line.length : 0;
        
        // Use the last column of the line to ensure decoration spans wrapped lines
        const endColumn = Math.max(1, lineLength + 1);
        
        if (lineMapping.Matched) {
            // Matched line - add gutter marker
            // Use range that spans the entire line (including wrapped parts) for proper glyph margin display
            const groupNames = lineMapping.Matches && lineMapping.Matches.length > 0
                ? lineMapping.Matches.map(m => m.GroupName || 'unnamed').join(', ')
                : 'matched';
            
            decorations.push({
                range: new monaco.Range(lineNumber, 1, lineNumber, endColumn),
                options: {
                    glyphMarginClassName: 'source-map-matched-gutter',
                    glyphMarginHoverMessage: { value: `Matched by group: ${groupNames}` },
                    glyphMarginLane: monaco.editor.GlyphMarginLane.Left,
                    stickiness: monaco.editor.TrackedRangeStickiness.NeverGrowsWhenTypingAtEdges
                }
            });
            
            // Add decorations for each match on this line
            if (lineMapping.Matches && lineMapping.Matches.length > 0) {
                lineMapping.Matches.forEach((match) => {
                    // Validate column positions
                    // Source map uses 0-indexed columns (StartCol inclusive, EndCol exclusive)
                    // Monaco uses 1-indexed columns (both start and end inclusive)
                    const startCol = Math.max(0, Math.min(match.StartCol, lineLength));
                    const endCol = Math.max(startCol, Math.min(match.EndCol, lineLength));
                    
                    // Convert to Monaco's 1-indexed system: startCol+1, endCol (since EndCol is exclusive in source map)
                    if (endCol > startCol && startCol >= 0) {
                        // Match decoration with click handler
                        const resultPath = match.ResultPath || match.GroupName || '';
                        const clickHandler = resultPath ? () => navigateToResultPath(resultPath) : null;
                        
                        decorations.push({
                            range: new monaco.Range(lineNumber, startCol + 1, lineNumber, endCol + 1),
                            options: {
                                className: 'source-map-match-highlight',
                                hoverMessage: { 
                                    value: `Group: ${match.GroupName || 'unnamed'}, Pattern: ${match.PatternIndex}${resultPath ? `, Path: ${resultPath}` : ''}\nClick to navigate to result` 
                                },
                                glyphMarginClassName: 'source-map-clickable',
                                stickiness: monaco.editor.TrackedRangeStickiness.NeverGrowsWhenTypingAtEdges
                            }
                        });
                        
                        // Variable decorations
                        if (match.Variables) {
                            Object.entries(match.Variables).forEach(([varName, varRange]) => {
                                // Source map uses 0-indexed columns (StartCol inclusive, EndCol exclusive)
                                // Monaco uses 1-indexed columns (both start and end inclusive)
                                const varStartCol = Math.max(0, Math.min(varRange.StartCol, lineLength));
                                const varEndCol = Math.max(varStartCol, Math.min(varRange.EndCol, lineLength));
                                
                                // Convert to Monaco's 1-indexed system: varStartCol+1, varEndCol+1
                                if (varEndCol > varStartCol && varStartCol >= 0) {
                                    decorations.push({
                                        range: new monaco.Range(lineNumber, varStartCol + 1, lineNumber, varEndCol + 1),
                                        options: {
                                            className: 'source-map-variable-highlight',
                                            hoverMessage: { value: `Variable: ${varName}` }
                                        }
                                    });
                                }
                            });
                        }
                    }
                });
            }
        } else {
            // Unmatched line - add gutter marker
            // Use range that spans the entire line (including wrapped parts) for proper glyph margin display
            decorations.push({
                range: new monaco.Range(lineNumber, 1, lineNumber, endColumn),
                options: {
                    glyphMarginClassName: 'source-map-unmatched-gutter',
                    glyphMarginHoverMessage: { value: 'No match found for this line' },
                    glyphMarginLane: monaco.editor.GlyphMarginLane.Left,
                    stickiness: monaco.editor.TrackedRangeStickiness.NeverGrowsWhenTypingAtEdges
                }
            });
        }
    });
    
    // Filter out any decorations that reference invalid line numbers (safety check)
    const validDecorations = decorations.filter(dec => {
        const range = dec.range;
        return range && range.startLineNumber >= 1 && range.startLineNumber <= maxLineCount;
    });
    
    // Apply decorations using deltaDecorations
    const decorationIds = inputEditor.deltaDecorations([], validDecorations);
    return decorationIds;
}

/**
 * Find result path at a given position in output editor
 * @param {Object} model - Monaco model
 * @param {Object} position - Position {lineNumber, column}
 * @returns {string|null} Result path or null
 */
function findResultPathAtPosition(model, position) {
    if (!model || !position) {
        return null;
    }
    
    const lineNumber = position.lineNumber;
    const column = position.column;
    const line = model.getLineContent(lineNumber);
    
    if (!line) {
        return null;
    }
    
    // Try to extract JSON key at this position
    // Look for quoted strings that might be keys
    const jsonKeyRegex = /"([^"]+)":/g;
    let match;
    let bestMatch = null;
    let bestKey = null;
    
    while ((match = jsonKeyRegex.exec(line)) !== null) {
        const keyStart = match.index + 1; // After opening quote
        const keyEnd = match.index + match[0].length - 3; // Before ":"
        
        if (column >= keyStart && column <= keyEnd) {
            const key = match[1];
            // Check if this key exists in our result path mapping
            if (sourceMapNavigationData.resultPathToMatches[key]) {
                return key;
            }
            // Store as potential match
            bestMatch = match;
            bestKey = key;
        }
    }
    
    // If we found a key but it's not in our mapping, try to find parent path
    if (bestKey) {
        // Try to build full path by looking at parent lines
        const fullPath = buildFullResultPath(model, lineNumber, bestKey);
        if (fullPath && sourceMapNavigationData.resultPathToMatches[fullPath]) {
            return fullPath;
        }
        
        // Try to match by checking if any result path contains this key
        // This handles nested paths like "cms:show-cable-modem.modem-entry*" where
        // the JSON has nested keys "cms" -> "show-cable-modem" -> "modem-entry*"
        for (const [resultPath, matches] of Object.entries(sourceMapNavigationData.resultPathToMatches)) {
            // Check if result path contains this key (handles nested paths)
            const pathParts = resultPath.split(/[.:]/);
            if (pathParts.includes(bestKey)) {
                return resultPath;
            }
            // Also check if key matches any part of the path
            if (resultPath.includes(bestKey) || bestKey.includes(resultPath.split(/[.:]/)[0])) {
                return resultPath;
            }
        }
        
        // Return the key anyway - might match a group name
        return bestKey;
    }
    
    // Try YAML key format
    const yamlKeyRegex = /^(\s*)([^:]+):/;
    const yamlMatch = line.match(yamlKeyRegex);
    if (yamlMatch && column <= yamlMatch[0].length) {
        const key = yamlMatch[2].trim();
        if (sourceMapNavigationData.resultPathToMatches[key]) {
            return key;
        }
        const fullPath = buildFullResultPath(model, lineNumber, key);
        if (fullPath && sourceMapNavigationData.resultPathToMatches[fullPath]) {
            return fullPath;
        }
        
        // Try to match by checking if any result path contains this key
        for (const [resultPath, matches] of Object.entries(sourceMapNavigationData.resultPathToMatches)) {
            const pathParts = resultPath.split(/[.:]/);
            if (pathParts.includes(key)) {
                return resultPath;
            }
        }
        
        return key;
    }
    
    return null;
}

/**
 * Build full result path by looking at parent structure
 * @param {Object} model - Monaco model
 * @param {number} lineNumber - Current line number
 * @param {string} key - Current key
 * @returns {string|null} Full path or null
 */
function buildFullResultPath(model, lineNumber, key) {
    // Try to find parent keys by looking at indentation
    const currentLine = model.getLineContent(lineNumber);
    const currentIndent = currentLine.match(/^\s*/)[0].length;
    
    // Look backwards for parent keys
    const pathParts = [key];
    
    for (let i = lineNumber - 1; i >= 1; i--) {
        const line = model.getLineContent(i);
        if (!line) continue;
        
        const indent = line.match(/^\s*/)[0].length;
        
        // If this line has less indent, it's a parent
        if (indent < currentIndent) {
            // Try to extract key from this line
            const jsonKeyMatch = line.match(/"([^"]+)":/);
            if (jsonKeyMatch) {
                pathParts.unshift(jsonKeyMatch[1]);
                
                // Check if this path exists in our mapping
                const fullPath = pathParts.join('.');
                if (sourceMapNavigationData.resultPathToMatches[fullPath]) {
                    return fullPath;
                }
                
                // Stop if we've gone too far up
                if (indent === 0) {
                    break;
                }
            }
        }
    }
    
    // Return the full path we built
    if (pathParts.length > 1) {
        return pathParts.join('.');
    }
    
    return null;
}

/**
 * Apply source map decorations to output editor (highlight clickable keys)
 * @param {Object} sourceMap - Source map data
 * @param {Object} resultData - Result data structure
 * @returns {string[]} Decoration IDs for later removal
 */
function applySourceMapOutputDecorations(sourceMap, resultData) {
    if (!outputEditor || !sourceMap || !resultData) {
        return [];
    }
    
    const model = outputEditor.getModel();
    if (!model) {
        return [];
    }
    
    const decorations = [];
    const content = model.getValue();
    const lines = content.split('\n');
    
    // Get all result paths that have matches
    const resultPaths = Object.keys(sourceMapNavigationData.resultPathToMatches);
    
    // For each result path, find it in the output and add decoration
    resultPaths.forEach(resultPath => {
        // Skip paths that contain asterisks (formatters) - we'll find the base path instead
        const pathWithoutFormatters = resultPath.replace(/\*+$/, '');
        
        // Check if this is a variable path (e.g., "version-info*.file-name")
        const pathParts = pathWithoutFormatters.split('.');
        const isVariablePath = pathParts.length > 1;
        const searchKey = isVariablePath ? pathParts[pathParts.length - 1] : pathWithoutFormatters;
        
        // Escape the search key for regex
        const escapedKey = searchKey.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
        const keyRegex = new RegExp(`"${escapedKey}"\\s*:`, 'g');
        
        // Find ALL occurrences of this key in the output (not just the first one)
        for (let i = 0; i < lines.length; i++) {
            const line = lines[i];
            
            // Reset regex lastIndex to search from start of line
            keyRegex.lastIndex = 0;
            let match;
            
            // Find all matches on this line (though typically there's only one per line)
            while ((match = keyRegex.exec(line)) !== null) {
                // Match is like "booted-from": - we want to highlight just "booted-from" (without quotes)
                // match.index points to the opening quote (0-indexed string position)
                // match[0] is the full match including quotes and colon (e.g., "booted-from":)
                // Monaco columns are 1-indexed and ranges are inclusive on both ends
                // Find the closing quote position in the matched string
                const matchStr = match[0];
                const closingQuoteIndex = matchStr.indexOf('"', 1); // Find closing quote (skip opening quote at index 0)
                if (closingQuoteIndex > 0) {
                    // Highlight from opening quote to closing quote (inclusive)
                    // match.index is 0-indexed string position of opening quote
                    // closingQuoteIndex is 0-indexed position of closing quote within the matched string
                    // In the line, closing quote is at: match.index + closingQuoteIndex (0-indexed)
                    // Monaco columns are 1-indexed, so add 1
                    const keyStart = match.index + 1; // Opening quote (Monaco 1-indexed)
                    // closingQuoteIndex is position within match, add 1 to get position after closing quote to include it
                    const keyEnd = match.index + closingQuoteIndex + 2; // Position after closing quote (Monaco 1-indexed, inclusive)
                    decorations.push({
                        range: new monaco.Range(i + 1, keyStart, i + 1, keyEnd),
                        options: {
                            className: 'source-map-clickable-key',
                            hoverMessage: { value: `Click to navigate to input (${resultPath})` },
                            stickiness: monaco.editor.TrackedRangeStickiness.NeverGrowsWhenTypingAtEdges
                        }
                    });
                }
            }
        }
    });
    
    // Apply decorations
    const decorationIds = outputEditor.deltaDecorations([], decorations);
    return decorationIds;
}

/**
 * Clear source map decorations from input editor
 * @param {string[]} decorationIds - Decoration IDs to remove
 */
function clearSourceMapDecorations(decorationIds) {
    if (!inputEditor || !decorationIds || decorationIds.length === 0) {
        return;
    }
    
    inputEditor.deltaDecorations(decorationIds, []);
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

