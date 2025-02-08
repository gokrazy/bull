// To re-generate the CodeMirror editor JavaScript bundle with the currently
// used versions, run regenerate.sh.
//
// To update to a new CodeMirror version, run:
//
// (cd third_party/codemirror && npm install codemirror)
// (cd third_party/codemirror && npm install codemirror/lang-markdown)

import {EditorView} from "codemirror"
import {markdown} from "@codemirror/lang-markdown"

import {keymap, highlightSpecialChars, drawSelection, highlightActiveLine, dropCursor,
        rectangularSelection, crosshairCursor,
        lineNumbers, highlightActiveLineGutter} from "@codemirror/view"
import {EditorState} from "@codemirror/state"
import {defaultHighlightStyle, syntaxHighlighting, indentOnInput, bracketMatching,
        foldGutter, foldKeymap} from "@codemirror/language"
import {defaultKeymap, history, historyKeymap, indentWithTab} from "@codemirror/commands"
import {searchKeymap, highlightSelectionMatches} from "@codemirror/search"
import {lintKeymap} from "@codemirror/lint"

let bullSetup = [
    lineNumbers(),
    highlightActiveLineGutter(),
    highlightSpecialChars(),
    history(),
    foldGutter(),
    drawSelection(),
    dropCursor(),
    EditorState.allowMultipleSelections.of(true),
    indentOnInput(),
    syntaxHighlighting(defaultHighlightStyle, {fallback: true}),
    bracketMatching(),

    // I do not like this behavior.
    // closeBrackets(),

    // The autocompletion extension installs an alt+p / alt+z
    // keyboard shortcut, which prevents me from entering
    // ~ or ` when using neo-layout.org.
    // autocompletion(),

    rectangularSelection(),
    crosshairCursor(),
    highlightActiveLine(),
    highlightSelectionMatches(),
    keymap.of([
	...defaultKeymap,
	...foldKeymap,
	...historyKeymap,
	...lintKeymap,
	...searchKeymap,
	// TODO: document why indentWithTab breaks with ...
	// prepended, but others need it?!
	indentWithTab,
    ]),
]

declare var BullMarkdown: string

let editor = new EditorView({
    doc: BullMarkdown,
    extensions: [
	bullSetup,
	markdown(),
	EditorView.lineWrapping
    ],
    parent: document.getElementById('cm-goes-here')
})

editor.focus();

// Inject the editor content into the <form> before submit
document.getElementById('bull-save').onclick = function(_unused) {
    var bullMarkdown = <HTMLTextAreaElement>document.getElementById('bull-markdown');
    bullMarkdown.value = editor.state.doc.toString();
}
