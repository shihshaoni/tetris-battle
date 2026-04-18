const vscode = require('vscode');
const fs = require('fs');
const path = require('path');

function activate(context) {
  let disposable = vscode.commands.registerCommand('tetris.battle', () => {
    const config = vscode.workspace.getConfiguration('tetrisBattle');
    const serverUrl = config.get('serverUrl', 'wss://tetris-battle.fly.dev/ws');
    const playerName = config.get('playerName', '') || vscode.env.machineId.slice(0, 8);

    const panel = vscode.window.createWebviewPanel(
      'tetris-battle',
      '🎮 Tetris Battle',
      vscode.ViewColumn.One,
      {
        enableScripts: true,
        retainContextWhenHidden: true
      }
    );

    const htmlPath = path.join(context.extensionPath, 'media', 'battle.html');
    let html = fs.readFileSync(htmlPath, 'utf8');
    // Inject config - replaceAll to catch every occurrence
    html = html.replaceAll('__SERVER_URL__', serverUrl);
    html = html.replaceAll('__PLAYER_NAME__', playerName);
    panel.webview.html = html;
  });

  context.subscriptions.push(disposable);
}

function deactivate() {}

module.exports = { activate, deactivate };
