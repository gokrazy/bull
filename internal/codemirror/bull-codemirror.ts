// -*- tab-width: 4; indent-tabs-mode: nil; -*-
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

declare var BullURLPrefix: string
declare var BullURLBullPrefix: string
declare var BullPageName: string
declare var BullMarkdown: string

function filesFromClipboard(dt: DataTransfer): File[] {
  return Array.from(dt.items)
    .filter((item) => item.kind === "file")
    .map((item) => item.getAsFile())
    .filter((f): f is File => f !== null)
}

function replacePlaceholder(view: EditorView, find: string, replace: string) {
    const idx = view.state.doc.toString().indexOf(find)
    if (idx < 0) {
        return
    }
    view.dispatch(view.state.update({
        changes: {
            from: idx,
            to: idx + find.length,
            insert: replace,
        },
    }))
}

// Produce different placeholders even if the file name is identical.
var uploadCounter = 0;

function placeholderFor(f: File): string {
    uploadCounter++
    return '[(uploading) ' + f.name + '](#bull-upload-' + uploadCounter + ')'
}

function bullUpload(files: File[], view: EditorView, pos: number) {
    var placeholdersText = '';
    var placeholders: string[] = [];
    for (let i = 0; i < files.length; i++) {
        const f = files[i]
        const placeholder = placeholderFor(f)
        placeholders[i] = placeholder
        placeholdersText += placeholder + ' '
    }
    // insert placeholders: describe what will be uploaded
    view.dispatch(view.state.update({
        changes: {from: pos, insert: placeholdersText},
        selection: {anchor: pos + placeholdersText.length},
    }))

    for (let i = 0; i < files.length; i++) {
        const f = files[i]
        const placeholder = placeholders[i]
        var uploadURL = new URL(document.location.href);
        uploadURL.pathname = BullURLBullPrefix + "upload/" + BullPageName
        uploadURL.searchParams.append('filename', f.name)
        uploadURL.searchParams.append('lastmodified', f.lastModified.toString())
        uploadURL.searchParams.append('size', f.size.toString())
        uploadURL.searchParams.append('type', f.type)
        fetch(uploadURL.toString(), {
            method: "POST",
            body: f,
        })
            .then(function(resp) {
                if (!resp.ok) {
                    replacePlaceholder(view, placeholder, "<!-- upload failed: " + resp.status + " " + resp.statusText + " -->")
                    return
                }
                resp.json().then(respb => {
                    var u = new URL(document.location.href);
                    u.pathname = BullURLPrefix + respb.path
                    if (f.type.startsWith('image/')) {
                        replacePlaceholder(view, placeholder, "![](" + u.pathname.toString() + ")")
                    } else {
                        replacePlaceholder(view, placeholder, "[" + f.name + "](" + u.pathname.toString() + ")")
                    }
                })
            })
            .catch(function(err) {
                replacePlaceholder(view, placeholder, "<!-- upload failed: " + err + " -->")
            })
    }

    return true
}

function bullFileDrop(event: any, view: EditorView) {
    const pos = view.posAtCoords({x: event.clientX, y: event.clientY})
    if (pos === null) {
        alert('BUG: view.posAtCoords() returned null?!')
        return false
    }

    const files = filesFromClipboard(event.dataTransfer)
    if (files.length === 0) {
        return false
    }
    return bullUpload(files, view, pos)
}

function bullPaste(event: any, view: EditorView) {
    const pos = view.state.selection.main.head
    const files = filesFromClipboard(event.clipboardData)
    if (files.length === 0) {
        // This paste contained text, no images/files.
        return false
    }
    return bullUpload(files, view, pos)
}

const bullFileUpload = EditorView.domEventHandlers({
    drop(event, view) {
	return bullFileDrop(event, view)
    },
    paste(event, view) {
	return bullPaste(event, view)
    },
})

let editor = new EditorView({
    doc: BullMarkdown,
    extensions: [
	bullSetup,
	markdown(),
	EditorView.lineWrapping,
	bullFileUpload,
    ],
    parent: document.getElementById('cm-goes-here')!
})

editor.focus();

// Inject the editor content into the <form> before submit
document.getElementById('bull-save')!.onclick = function(_unused) {
    var bullMarkdown = <HTMLTextAreaElement>document.getElementById('bull-markdown');
    bullMarkdown.value = editor.state.doc.toString();
}
