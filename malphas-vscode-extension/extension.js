const vscode = require('vscode');
const { LanguageClient, TransportKind } = require('vscode-languageclient/node');

let client;

function activate(context) {
    // Always show that we're activating
    vscode.window.showInformationMessage('Malphas extension is activating...', 'OK');
    console.log('[Malphas] Extension activate() called');
    
    // Configuration for the language server
    const serverOptions = {
        run: {
            command: 'malphas',
            args: ['lsp'],
            transport: TransportKind.stdio
        },
        debug: {
            command: 'malphas',
            args: ['lsp'],
            transport: TransportKind.stdio
        }
    };

    // Client options
    const clientOptions = {
        documentSelector: [{ scheme: 'file', language: 'malphas' }],
        synchronize: {
            fileEvents: vscode.workspace.createFileSystemWatcher('**/*.mal')
        }
    };

    // Create the language client
    client = new LanguageClient(
        'malphas',
        'Malphas Language Server',
        serverOptions,
        clientOptions
    );

    // Start the client
    client.start().then(() => {
        console.log('[Malphas] Language Server started successfully');
        vscode.window.showInformationMessage('Malphas Language Server started');
    }).catch(err => {
        console.error('[Malphas] Failed to start Language Server:', err);
        vscode.window.showErrorMessage('Malphas LSP Error: ' + err.message);
    });
}

function deactivate() {
    if (client) {
        return client.stop();
    }
}

module.exports = {
    activate,
    deactivate
};

