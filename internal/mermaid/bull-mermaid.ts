import mermaid from 'mermaid';

mermaid.initialize({ startOnLoad: false });

const blocks = document.querySelectorAll('pre > code.language-mermaid');
blocks.forEach(code => {
  const pre = code.parentElement!;
  pre.classList.add('mermaid');
  pre.textContent = code.textContent;
});

mermaid.run();
