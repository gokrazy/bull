// in the bull/third_party/codemirror directory:
//
// npm install codemirror
// npm install codemirror/lang-markdown

import {EditorView, basicSetup} from "codemirror"
import {markdown} from "@codemirror/lang-markdown"

let editor = new EditorView({
    doc: BullMarkdown,
    extensions: [ basicSetup, markdown() ],
    parent: document.getElementById('cm-goes-here')
})

// Inject the editor content into the <form> before submit
document.getElementById('bull-save').onclick = function(event) {
    document.getElementById('bull-markdown').value = editor.state.doc.toString();
}
