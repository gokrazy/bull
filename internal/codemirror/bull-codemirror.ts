// To re-generate the CodeMirror editor JavaScript bundle with the currently
// used versions, run regenerate.sh.
//
// To update to a new CodeMirror version, run:
//
// (cd third_party/codemirror && npm install codemirror)
// (cd third_party/codemirror && npm install codemirror/lang-markdown)

import {EditorView} from "codemirror"
import {markdown} from "@codemirror/lang-markdown"

import {
    crosshairCursor,
    drawSelection,
    dropCursor,
    highlightActiveLine,
    highlightActiveLineGutter,
    highlightSpecialChars,
    keymap,
    lineNumbers,
    rectangularSelection
} from "@codemirror/view"
import {EditorState} from "@codemirror/state"
import {
    bracketMatching,
    defaultHighlightStyle,
    foldGutter,
    foldKeymap,
    indentOnInput,
    syntaxHighlighting
} from "@codemirror/language"
import {defaultKeymap, history, historyKeymap, indentWithTab} from "@codemirror/commands"
import {highlightSelectionMatches, searchKeymap} from "@codemirror/search"
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

async function uploadFile(file: File): Promise<{}> {
    // TODO: better way to do this? Seems like the editor doesn't generally know what page it's on
    const uploadUrl = window.location.href.replace("/_bull/edit/", "/_bull/upload/");
    const formData = new FormData();

    formData.append('file', file);

    return fetch(uploadUrl, {
        method: "POST",
        body: formData,
    }).then(res => res.json());
}

declare var BullMarkdown: string


const eventHandlers = EditorView.domEventHandlers({
    paste(event, view) {
        let files: File[] = []
        for (let item of event.clipboardData.items) {
            if (item.kind == "file" && item.type.startsWith("image")) {
                files.push(item.getAsFile())
            }
        }
        if (files.length == 0) {
            return
        }
        // If something is selected, replace it with the pasted items
        let start = view.state.selection.main.from
        let end = view.state.selection.main.to
        view.dispatch({changes: {
                from: start,
                to: end,
                insert: files.map((f) => "[Uploading " + f.name + "]()").join("\n") + "\n"
            },})

        files.forEach((file) => {
            uploadFile(file).then(uploaded => {
                const start = view.state.doc.toString().indexOf("[Uploading " + file.name + "]()")
                const end = start + ("[Uploading " + file.name + "]()").length
                const body = "![" + file.name + "](" + uploaded['savedFilename'] + ")"
                view.dispatch({changes: {
                        from: start,
                        to: end,
                        insert: body,
                    }})
            });
        })
    },
    drop(event, view) {
        let files: File[] = []
        for (let item of event.dataTransfer.items) {
            if (item.kind == "file" && item.type.startsWith("image")) {
                files.push(item.getAsFile())
            }
        }
        // Keep regular behavior when dropping non-files
        if (!files) return;
        event.preventDefault();

        const messages = files.map((f) => "[Uploading " + f.name + "]()").join("\n");

        // If something is selected, replace it with the upload messages
        // otherwise, put them at the position where the items were dropped
        let start = view.state.selection.main.from
        let end = view.state.selection.main.to
        if (start == end) {
            start = view.posAtCoords({x: event.x, y: event.y})
            end = start
        }
        view.dispatch({changes: {
                from: start,
                to: end,
                insert: messages + "\n",
            },})

        files.forEach((file) => {
            uploadFile(file).then(uploaded => {
                const start = view.state.doc.toString().indexOf("[Uploading " + file.name + "]()")
                const end = start + ("[Uploading " + file.name + "]()").length
                const body = "![" + file.name + "](" + uploaded['savedFilename'] + ")"
                view.dispatch({
                    changes: {
                        from: start,
                        to: end,
                        insert: body,
                    }
                })
            });
        })}
})

let editor = new EditorView({
    doc: BullMarkdown,
    extensions: [
	bullSetup,
	markdown(),
	EditorView.lineWrapping,
        eventHandlers
    ],
    parent: document.getElementById('cm-goes-here')
})

editor.focus();

// Inject the editor content into the <form> before submit
document.getElementById('bull-save').onclick = function(_unused) {
    var bullMarkdown = <HTMLTextAreaElement>document.getElementById('bull-markdown');
    bullMarkdown.value = editor.state.doc.toString();
}
