import React, { useState, useEffect, useRef } from 'react';
import FileTree from './components/FileTree';
import Editor from './components/Editor';
import Terminal from './components/Terminal';
import './App.css';

function App() {
  const [selectedFile, setSelectedFile] = useState(null);
  const [fileContent, setFileContent] = useState('');
  const [currentPath, setCurrentPath] = useState('/');
  const [sidebarWidth, setSidebarWidth] = useState(250);
  const [terminalHeight, setTerminalHeight] = useState(250);
  const wsRef = useRef(null);
  const isResizingSidebar = useRef(false);
  const isResizingTerminal = useRef(false);

  const handleFileSelect = async (file) => {
    if (file.type === 'file') {
      setSelectedFile(file);
      // Fetch file content from API
      try {
        const response = await fetch(`/api/files/read?path=${encodeURIComponent(file.path)}`);
        const data = await response.json();
        setFileContent(data.content || '');
      } catch (error) {
        console.error('Error reading file:', error);
        setFileContent('');
      }
    }
  };

  const handleFileSave = async (content) => {
    if (!selectedFile) return;

    try {
      await fetch('/api/files/write', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          path: selectedFile.path,
          content: content,
        }),
      });
    } catch (error) {
      console.error('Error saving file:', error);
    }
  };

  // Handle sidebar resize
  const handleSidebarMouseDown = (e) => {
    isResizingSidebar.current = true;
    e.preventDefault();
  };

  const handleMouseMove = (e) => {
    if (isResizingSidebar.current) {
      const newWidth = e.clientX;
      if (newWidth >= 150 && newWidth <= 600) {
        setSidebarWidth(newWidth);
      }
    }
    if (isResizingTerminal.current) {
      const newHeight = window.innerHeight - e.clientY;
      if (newHeight >= 100 && newHeight <= window.innerHeight - 200) {
        setTerminalHeight(newHeight);
      }
    }
  };

  const handleMouseUp = () => {
    isResizingSidebar.current = false;
    isResizingTerminal.current = false;
  };

  // Handle terminal resize
  const handleTerminalMouseDown = (e) => {
    isResizingTerminal.current = true;
    e.preventDefault();
  };

  useEffect(() => {
    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
  }, []);

  return (
    <div className="app">
      <div className="sidebar" style={{ width: `${sidebarWidth}px` }}>
        <div className="sidebar-header">Explorer</div>
        <FileTree
          currentPath={currentPath}
          onFileSelect={handleFileSelect}
          selectedFile={selectedFile}
          wsRef={wsRef}
        />
      </div>
      <div className="resizer resizer-vertical" onMouseDown={handleSidebarMouseDown}></div>
      <div className="main-content">
        <div className="editor-container" style={{ height: `calc(100% - ${terminalHeight}px)` }}>
          <Editor
            file={selectedFile}
            content={fileContent}
            onSave={handleFileSave}
          />
        </div>
        <div className="resizer resizer-horizontal" onMouseDown={handleTerminalMouseDown}></div>
        <div className="terminal-container" style={{ height: `${terminalHeight}px` }}>
          <Terminal wsRef={wsRef} />
        </div>
      </div>
    </div>
  );
}

export default App;
