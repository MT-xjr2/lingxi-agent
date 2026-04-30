import { useState, useRef, useEffect, useCallback } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

// ─── Skills 管理页面 ──────────────────────────────────────────────
export default function SkillsPage({ onBack }) {
  const [skills, setSkills] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState('list'); // 'list' | 'upload' | 'generate'

  // AI 生成相关状态
  const [genDesc, setGenDesc] = useState('');
  const [genLoading, setGenLoading] = useState(false);
  const [genLogs, setGenLogs] = useState([]); // 生成过程日志
  const [genPreview, setGenPreview] = useState(null); // { skillName, tmpDir, files: [{path, content}] }
  const [editingFile, setEditingFile] = useState(null); // 当前编辑的文件 path
  const [editedFiles, setEditedFiles] = useState({}); // 用户修改的文件内容

  // 上传相关状态
  const [uploadDragging, setUploadDragging] = useState(false);
  const [uploadLoading, setUploadLoading] = useState(false);
  const [uploadError, setUploadError] = useState('');
  const [uploadResults, setUploadResults] = useState([]);
  const fileInputRef = useRef(null);

  const logRef = useRef(null);

  useEffect(() => {
    fetchSkills();
  }, []);

  useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [genLogs]);

  const fetchSkills = async () => {
    setLoading(true);
    try {
      const r = await fetch('/api/skills', { credentials: 'include' });
      const data = await r.json();
      setSkills(data || []);
    } finally {
      setLoading(false);
    }
  };

  // ── 安装 / 卸载 ────────────────────────────────────────────────
  const handleInstall = async (skill) => {
    setSkills(prev => prev.map(s => s.id === skill.id ? { ...s, _loading: true } : s));
    try {
      await fetch(`/api/skills/${skill.id}/install`, { method: 'POST', credentials: 'include' });
      setSkills(prev => prev.map(s => s.id === skill.id ? { ...s, installed: true, _loading: false } : s));
    } catch {
      setSkills(prev => prev.map(s => s.id === skill.id ? { ...s, _loading: false } : s));
    }
  };

  const handleUninstall = async (skill) => {
    setSkills(prev => prev.map(s => s.id === skill.id ? { ...s, _loading: true } : s));
    try {
      await fetch(`/api/skills/${skill.id}/uninstall`, { method: 'POST', credentials: 'include' });
      setSkills(prev => prev.map(s => s.id === skill.id ? { ...s, installed: false, _loading: false } : s));
    } catch {
      setSkills(prev => prev.map(s => s.id === skill.id ? { ...s, _loading: false } : s));
    }
  };

  const handleDelete = async (skill) => {
    if (!confirm(`确认删除 skill "${skill.name}"？此操作不可恢复。`)) return;
    await fetch(`/api/skills/${skill.id}`, { method: 'DELETE', credentials: 'include' });
    setSkills(prev => prev.filter(s => s.id !== skill.id));
  };

  // ── 上传 zip（支持批量）──────────────────────────────────────────
  const handleUploadFiles = async (files) => {
    const zipFiles = Array.from(files).filter(f => f.name.endsWith('.zip'));
    if (zipFiles.length === 0) {
      setUploadError('请上传 .zip 格式的文件');
      return;
    }
    setUploadError('');
    setUploadResults([]);
    setUploadLoading(true);

    try {
      if (zipFiles.length === 1) {
        const form = new FormData();
        form.append('file', zipFiles[0]);
        const r = await fetch('/api/skills/upload', { method: 'POST', credentials: 'include', body: form });
        const data = await r.json();
        if (!r.ok) {
          setUploadError(data.error || '上传失败');
          return;
        }
        setSkills(prev => {
          const exists = prev.find(s => s.id === data.id);
          if (exists) return prev.map(s => s.id === data.id ? data : s);
          return [data, ...prev];
        });
        setUploadResults([{ filename: zipFiles[0].name, success: true }]);
      } else {
        const form = new FormData();
        zipFiles.forEach(f => form.append('files', f));
        const r = await fetch('/api/skills/batch-upload', { method: 'POST', credentials: 'include', body: form });
        const data = await r.json();
        if (!r.ok) {
          setUploadError(data.error || '批量上传失败');
          return;
        }
        const results = data.results || [];
        setUploadResults(results);
        results.forEach(res => {
          if (res.success && res.skill) {
            setSkills(prev => {
              const exists = prev.find(s => s.id === res.skill.id);
              if (exists) return prev.map(s => s.id === res.skill.id ? res.skill : s);
              return [res.skill, ...prev];
            });
          }
        });
      }
    } catch (e) {
      setUploadError('上传失败: ' + e.message);
    } finally {
      setUploadLoading(false);
    }
  };

  const handleDrop = (e) => {
    e.preventDefault();
    setUploadDragging(false);
    handleUploadFiles(e.dataTransfer.files);
  };

  // ── AI 生成 ────────────────────────────────────────────────────
  const handleGenerate = useCallback(async () => {
    if (!genDesc.trim() || genLoading) return;
    setGenLoading(true);
    setGenLogs([]);
    setGenPreview(null);
    setEditedFiles({});

    try {
      const resp = await fetch('/api/skills/generate/stream', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ description: genDesc }),
      });

      if (!resp.ok) {
        const err = await resp.json();
        setGenLogs(prev => [...prev, { type: 'error', text: err.error || '生成失败' }]);
        return;
      }

      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      let buf = '';
      let currentEvent = 'text';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        const lines = buf.split('\n');
        buf = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('event: ')) {
            currentEvent = line.slice(7).trim();
          } else if (line.startsWith('data: ')) {
            const raw = line.slice(6);
            if (raw === '[DONE]') continue;

            let chunk = raw;
            try { chunk = JSON.parse(raw); } catch { /* keep raw */ }

            if (currentEvent === 'text' && chunk) {
              setGenLogs(prev => {
                const last = prev[prev.length - 1];
                if (last?.type === 'text') {
                  return [...prev.slice(0, -1), { type: 'text', text: last.text + chunk }];
                }
                return [...prev, { type: 'text', text: chunk }];
              });
            } else if (currentEvent === 'tool_start') {
              setGenLogs(prev => [...prev, { type: 'tool', text: `调用工具: ${chunk.name || ''}`, done: false }]);
            } else if (currentEvent === 'tool_end') {
              setGenLogs(prev => {
                const last = prev[prev.length - 1];
                if (last?.type === 'tool') return [...prev.slice(0, -1), { ...last, done: true }];
                return prev;
              });
            } else if (currentEvent === 'preview') {
              // 生成完成，收到预览数据
              setGenPreview(chunk);
              // 初始化 editedFiles
              const initFiles = {};
              (chunk.files || []).forEach(f => { initFiles[f.path] = f.content; });
              setEditedFiles(initFiles);
              if (chunk.files?.length > 0) {
                setEditingFile(chunk.files[0].path);
              }
            } else if (currentEvent === 'error') {
              setGenLogs(prev => [...prev, { type: 'error', text: chunk }]);
            }
            currentEvent = 'text';
          }
        }
      }
    } catch (e) {
      setGenLogs(prev => [...prev, { type: 'error', text: e.message }]);
    } finally {
      setGenLoading(false);
    }
  }, [genDesc, genLoading]);

  // ── 确认提交生成的 skill ───────────────────────────────────────
  const handleConfirm = async () => {
    if (!genPreview) return;
    setGenLoading(true);
    try {
      const r = await fetch('/api/skills/generate/confirm', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          tmpDir: genPreview.tmpDir,
          skillName: genPreview.skillName,
          files: editedFiles,
        }),
      });
      const data = await r.json();
      if (!r.ok) {
        alert(data.error || '提交失败');
        return;
      }
      setSkills(prev => {
        const exists = prev.find(s => s.id === data.id);
        if (exists) return prev.map(s => s.id === data.id ? data : s);
        return [data, ...prev];
      });
      // 重置状态
      setGenPreview(null);
      setGenLogs([]);
      setGenDesc('');
      setEditedFiles({});
      setActiveTab('list');
    } catch (e) {
      alert('提交失败: ' + e.message);
    } finally {
      setGenLoading(false);
    }
  };

  const handleCancelPreview = () => {
    setGenPreview(null);
    setGenLogs([]);
    setEditedFiles({});
  };

  // ── 渲染 ───────────────────────────────────────────────────────
  return (
    <div className="skills-page">
      {/* 顶部导航 */}
      <div className="skills-header">
        <button className="skills-back-btn" onClick={onBack}>
          ← 返回对话
        </button>
        <h2 className="skills-title">技能管理</h2>
        <div className="skills-tabs">
          <button className={`skills-tab${activeTab === 'list' ? ' active' : ''}`} onClick={() => setActiveTab('list')}>
            已有 Skills
          </button>
          <button className={`skills-tab${activeTab === 'generate' ? ' active' : ''}`} onClick={() => setActiveTab('generate')}>
            AI 生成
          </button>
          <button className={`skills-tab${activeTab === 'upload' ? ' active' : ''}`} onClick={() => setActiveTab('upload')}>
            上传压缩包
          </button>
        </div>
      </div>

      <div className="skills-content">
        {/* ── 列表 Tab ── */}
        {activeTab === 'list' && (
          <div className="skills-list-tab">
            {loading ? (
              <div className="skills-loading">加载中...</div>
            ) : skills.length === 0 ? (
              <div className="skills-empty">
                <div className="skills-empty-icon">⚡</div>
                <p>还没有 Skills，去 AI 生成或上传一个吧</p>
              </div>
            ) : (
              <div className="skills-grid">
                {skills.map(skill => (
                  <SkillCard
                    key={skill.id}
                    skill={skill}
                    onInstall={handleInstall}
                    onUninstall={handleUninstall}
                    onDelete={handleDelete}
                  />
                ))}
              </div>
            )}
          </div>
        )}

        {/* ── AI 生成 Tab ── */}
        {activeTab === 'generate' && (
          <div className="skills-generate-tab">
            {!genPreview ? (
              <>
                <div className="gen-form">
                  <label className="gen-label">描述你想要的 Skill 功能</label>
                  <textarea
                    className="gen-textarea"
                    value={genDesc}
                    onChange={e => setGenDesc(e.target.value)}
                    placeholder={`例如：
• 帮我创建一个 MySQL 数据库运维 skill，能查询慢查询、分析表结构、优化索引
• 创建一个 Git 工作流 skill，支持规范化提交、分支管理、代码审查流程
• 我需要一个 Python 代码质量检查 skill，能运行 lint、格式化、类型检查`}
                    rows={6}
                    disabled={genLoading}
                  />
                  <button
                    className={`gen-btn${genLoading ? ' loading' : ''}`}
                    onClick={handleGenerate}
                    disabled={genLoading || !genDesc.trim()}
                  >
                    {genLoading ? <><span className="gen-spinner" />生成中...</> : '✨ 开始生成'}
                  </button>
                </div>

                {genLogs.length > 0 && (
                  <div className="gen-logs" ref={logRef}>
                    <div className="gen-logs-title">生成过程</div>
                    {genLogs.map((log, i) => (
                      <GenLogItem key={i} log={log} />
                    ))}
                    {genLoading && (
                      <div className="gen-log-cursor">▋</div>
                    )}
                  </div>
                )}
              </>
            ) : (
              /* ── 预览 & 编辑 ── */
              <div className="gen-preview">
                <div className="gen-preview-header">
                  <div>
                    <h3 className="gen-preview-title">预览生成的 Skill: <code>{genPreview.skillName}</code></h3>
                    <p className="gen-preview-hint">你可以在下方编辑文件内容，确认无误后点击「保存并发布」</p>
                  </div>
                  <div className="gen-preview-actions">
                    <button className="gen-cancel-btn" onClick={handleCancelPreview} disabled={genLoading}>
                      重新生成
                    </button>
                    <button
                      className={`gen-confirm-btn${genLoading ? ' loading' : ''}`}
                      onClick={handleConfirm}
                      disabled={genLoading}
                    >
                      {genLoading ? <><span className="gen-spinner" />保存中...</> : '✓ 保存并发布'}
                    </button>
                  </div>
                </div>

                <div className="gen-editor">
                  {/* 文件树 */}
                  <div className="gen-file-tree">
                    <div className="gen-file-tree-title">文件结构</div>
                    {genPreview.files.map(f => (
                      <button
                        key={f.path}
                        className={`gen-file-item${editingFile === f.path ? ' active' : ''}`}
                        onClick={() => setEditingFile(f.path)}
                      >
                        <span className="gen-file-icon">{getFileIcon(f.path)}</span>
                        <span className="gen-file-path">{f.path}</span>
                      </button>
                    ))}
                  </div>

                  {/* 文件编辑器 */}
                  <div className="gen-file-editor">
                    {editingFile && (
                      <>
                        <div className="gen-editor-filename">{editingFile}</div>
                        {editingFile.endsWith('.md') ? (
                          <div className="gen-editor-split">
                            <textarea
                              className="gen-editor-textarea"
                              value={editedFiles[editingFile] || ''}
                              onChange={e => setEditedFiles(prev => ({ ...prev, [editingFile]: e.target.value }))}
                            />
                            <div className="gen-editor-preview">
                              <ReactMarkdown remarkPlugins={[remarkGfm]}>
                                {editedFiles[editingFile] || ''}
                              </ReactMarkdown>
                            </div>
                          </div>
                        ) : (
                          <textarea
                            className="gen-editor-textarea full"
                            value={editedFiles[editingFile] || ''}
                            onChange={e => setEditedFiles(prev => ({ ...prev, [editingFile]: e.target.value }))}
                          />
                        )}
                      </>
                    )}
                  </div>
                </div>
              </div>
            )}
          </div>
        )}

        {/* ── 上传 Tab ── */}
        {activeTab === 'upload' && (
          <div className="skills-upload-tab">
            <div className="upload-guide">
              <h3>Skill 压缩包结构要求</h3>
              <pre className="upload-guide-code">{`<skill-name>/
├── SKILL.md          # 【必需】包含 frontmatter 和指令
├── scripts/          # 【可选】辅助脚本
├── references/       # 【可选】参考文档
└── assets/           # 【可选】资源文件`}</pre>
              <p className="upload-guide-hint">SKILL.md 必须包含 <code>name</code> 和 <code>description</code> frontmatter</p>
            </div>

            <div
              className={`upload-zone${uploadDragging ? ' dragging' : ''}${uploadLoading ? ' loading' : ''}`}
              onDragOver={e => { e.preventDefault(); setUploadDragging(true); }}
              onDragLeave={() => setUploadDragging(false)}
              onDrop={handleDrop}
              onClick={() => !uploadLoading && fileInputRef.current?.click()}
            >
              <input
                ref={fileInputRef}
                type="file"
                accept=".zip"
                multiple
                style={{ display: 'none' }}
                onChange={e => handleUploadFiles(e.target.files)}
              />
              {uploadLoading ? (
                <div className="upload-loading">
                  <span className="gen-spinner large" />
                  <span>上传中...</span>
                </div>
              ) : (
                <>
                  <div className="upload-icon">📦</div>
                  <div className="upload-text">拖拽一个或多个 .zip 文件到此处，或点击选择</div>
                  <div className="upload-hint">支持批量上传 · 仅支持 .zip 格式</div>
                </>
              )}
            </div>

            {uploadError && (
              <div className="upload-error">{uploadError}</div>
            )}

            {uploadResults.length > 0 && (
              <div className="upload-results">
                <div className="upload-results-title">
                  上传结果（{uploadResults.filter(r => r.success).length}/{uploadResults.length} 成功）
                </div>
                {uploadResults.map((r, i) => (
                  <div key={i} className={`upload-result-item${r.success ? ' success' : ' fail'}`}>
                    <span className="upload-result-icon">{r.success ? '✓' : '✕'}</span>
                    <span className="upload-result-name">{r.filename}</span>
                    {r.error && <span className="upload-result-error">{r.error}</span>}
                    {r.success && r.skill && <span className="upload-result-skill">{r.skill.name}</span>}
                  </div>
                ))}
                {uploadResults.every(r => r.success) && (
                  <button className="upload-results-done" onClick={() => { setUploadResults([]); setActiveTab('list'); }}>
                    查看已上传的 Skills →
                  </button>
                )}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// ── Skill 卡片 ────────────────────────────────────────────────────
function SkillCard({ skill, onInstall, onUninstall, onDelete }) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className={`skill-card${skill.installed ? ' installed' : ''}`}>
      <div className="skill-card-header">
        <div className="skill-card-info">
          <div className="skill-card-name">
            <code>{skill.name}</code>
            {skill.installed && <span className="skill-installed-badge">已安装</span>}
          </div>
          <div className="skill-card-desc">{skill.description || '暂无描述'}</div>
        </div>
        <div className="skill-card-actions">
          {skill.installed ? (
            <button
              className="skill-btn uninstall"
              onClick={() => onUninstall(skill)}
              disabled={skill._loading}
            >
              {skill._loading ? '...' : '卸载'}
            </button>
          ) : (
            <button
              className="skill-btn install"
              onClick={() => onInstall(skill)}
              disabled={skill._loading}
            >
              {skill._loading ? '...' : '安装'}
            </button>
          )}
          <button className="skill-btn delete" onClick={() => onDelete(skill)} title="删除">✕</button>
        </div>
      </div>
      <div className="skill-card-meta">
        <span>创建者: {skill.created_by || '-'}</span>
        <span>·</span>
        <span>{new Date(skill.created_at).toLocaleDateString('zh-CN')}</span>
        <button className="skill-expand-btn" onClick={() => setExpanded(o => !o)}>
          {expanded ? '收起 ▲' : '详情 ▼'}
        </button>
      </div>
      {expanded && (
        <div className="skill-card-detail">
          <div className="skill-detail-row">
            <span className="skill-detail-label">OSS 路径</span>
            <code className="skill-detail-value">{skill.oss_key}</code>
          </div>
        </div>
      )}
    </div>
  );
}

// ── 生成日志条目 ──────────────────────────────────────────────────
function GenLogItem({ log }) {
  if (log.type === 'tool') {
    return (
      <div className={`gen-log-tool${log.done ? ' done' : ''}`}>
        <span className={`gen-log-dot${log.done ? ' done' : ' pulse'}`} />
        <span>{log.text}</span>
      </div>
    );
  }
  if (log.type === 'error') {
    return <div className="gen-log-error">{log.text}</div>;
  }
  return <div className="gen-log-text">{log.text}</div>;
}

// ── 工具函数 ──────────────────────────────────────────────────────
function getFileIcon(path) {
  if (path.endsWith('.md')) return '📄';
  if (path.endsWith('.py')) return '🐍';
  if (path.endsWith('.sh')) return '⚙️';
  if (path.endsWith('.js') || path.endsWith('.ts')) return '📜';
  if (path.endsWith('.json')) return '{}';
  if (path.endsWith('.png') || path.endsWith('.jpg')) return '🖼️';
  return '📁';
}
