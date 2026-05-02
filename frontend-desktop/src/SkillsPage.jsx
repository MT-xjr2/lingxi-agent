import { useState, useRef, useEffect, useCallback } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { motion, AnimatePresence } from 'framer-motion';
import {
  Sparkles, Upload, Wand2, Download, Trash2, ChevronDown, ChevronRight,
  Loader2, CheckCircle2, AlertCircle, FileText, FolderOpen, Code2,
} from 'lucide-react';
import { Button, Card, Badge, Modal } from './ui/primitives';
import { cn } from './ui/cn';

function getFileIcon(path) {
  if (path.endsWith('.md')) return FileText;
  if (path.endsWith('.py') || path.endsWith('.sh')) return Code2;
  if (path.endsWith('.js') || path.endsWith('.ts')) return Code2;
  return FolderOpen;
}

export default function SkillsPage() {
  const [skills, setSkills] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState('list');

  const [genDesc, setGenDesc] = useState('');
  const [genLoading, setGenLoading] = useState(false);
  const [genLogs, setGenLogs] = useState([]);
  const [genPreview, setGenPreview] = useState(null);
  const [editingFile, setEditingFile] = useState(null);
  const [editedFiles, setEditedFiles] = useState({});

  const [uploadDragging, setUploadDragging] = useState(false);
  const [uploadLoading, setUploadLoading] = useState(false);
  const [uploadError, setUploadError] = useState('');
  const [uploadResults, setUploadResults] = useState([]);
  const fileInputRef = useRef(null);
  const logRef = useRef(null);

  useEffect(() => { fetchSkills(); }, []);
  useEffect(() => { if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight; }, [genLogs]);

  const fetchSkills = async () => {
    setLoading(true);
    try {
      const r = await fetch('/api/skills', { credentials: 'include' });
      const data = await r.json();
      setSkills(data || []);
    } finally { setLoading(false); }
  };

  const handleInstall = async (skill) => {
    setSkills(prev => prev.map(s => s.id === skill.id ? { ...s, _loading: true } : s));
    try {
      await fetch(`/api/skills/${skill.id}/install`, { method: 'POST', credentials: 'include' });
      setSkills(prev => prev.map(s => s.id === skill.id ? { ...s, installed: true, _loading: false } : s));
    } catch { setSkills(prev => prev.map(s => s.id === skill.id ? { ...s, _loading: false } : s)); }
  };

  const handleUninstall = async (skill) => {
    setSkills(prev => prev.map(s => s.id === skill.id ? { ...s, _loading: true } : s));
    try {
      await fetch(`/api/skills/${skill.id}/uninstall`, { method: 'POST', credentials: 'include' });
      setSkills(prev => prev.map(s => s.id === skill.id ? { ...s, installed: false, _loading: false } : s));
    } catch { setSkills(prev => prev.map(s => s.id === skill.id ? { ...s, _loading: false } : s)); }
  };

  const handleDelete = async (skill) => {
    if (!confirm(`确认删除 skill "${skill.name}"？此操作不可恢复。`)) return;
    await fetch(`/api/skills/${skill.id}`, { method: 'DELETE', credentials: 'include' });
    setSkills(prev => prev.filter(s => s.id !== skill.id));
  };

  const handleUploadFiles = async (files) => {
    const zipFiles = Array.from(files).filter(f => f.name.endsWith('.zip'));
    if (zipFiles.length === 0) { setUploadError('请上传 .zip 格式的文件'); return; }
    setUploadError('');
    setUploadResults([]);
    setUploadLoading(true);
    try {
      if (zipFiles.length === 1) {
        const form = new FormData();
        form.append('file', zipFiles[0]);
        const r = await fetch('/api/skills/upload', { method: 'POST', credentials: 'include', body: form });
        const data = await r.json();
        if (!r.ok) { setUploadError(data.error || '上传失败'); return; }
        setSkills(prev => { const exists = prev.find(s => s.id === data.id); return exists ? prev.map(s => s.id === data.id ? data : s) : [data, ...prev]; });
        setUploadResults([{ filename: zipFiles[0].name, success: true }]);
      } else {
        const form = new FormData();
        zipFiles.forEach(f => form.append('files', f));
        const r = await fetch('/api/skills/batch-upload', { method: 'POST', credentials: 'include', body: form });
        const data = await r.json();
        if (!r.ok) { setUploadError(data.error || '批量上传失败'); return; }
        const results = data.results || [];
        setUploadResults(results);
        results.forEach(res => {
          if (res.success && res.skill) {
            setSkills(prev => { const exists = prev.find(s => s.id === res.skill.id); return exists ? prev.map(s => s.id === res.skill.id ? res.skill : s) : [res.skill, ...prev]; });
          }
        });
      }
    } catch (e) { setUploadError('上传失败: ' + e.message); }
    finally { setUploadLoading(false); }
  };

  const handleDrop = (e) => { e.preventDefault(); setUploadDragging(false); handleUploadFiles(e.dataTransfer.files); };

  const handleGenerate = useCallback(async () => {
    if (!genDesc.trim() || genLoading) return;
    setGenLoading(true); setGenLogs([]); setGenPreview(null); setEditedFiles({});
    try {
      const resp = await fetch('/api/skills/generate/stream', { method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ description: genDesc }) });
      if (!resp.ok) { const err = await resp.json(); setGenLogs(prev => [...prev, { type: 'error', text: err.error || '生成失败' }]); return; }
      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      let buf = '', currentEvent = 'text';
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        const lines = buf.split('\n');
        buf = lines.pop() || '';
        for (const line of lines) {
          if (line.startsWith('event: ')) { currentEvent = line.slice(7).trim(); }
          else if (line.startsWith('data: ')) {
            const raw = line.slice(6);
            if (raw === '[DONE]') continue;
            let chunk = raw;
            try { chunk = JSON.parse(raw); } catch { /* keep raw */ }
            if (currentEvent === 'text' && chunk) {
              setGenLogs(prev => { const last = prev[prev.length - 1]; return last?.type === 'text' ? [...prev.slice(0, -1), { type: 'text', text: last.text + chunk }] : [...prev, { type: 'text', text: chunk }]; });
            } else if (currentEvent === 'tool_start') { setGenLogs(prev => [...prev, { type: 'tool', text: `调用工具: ${chunk.name || ''}`, done: false }]); }
            else if (currentEvent === 'tool_end') { setGenLogs(prev => { const last = prev[prev.length - 1]; return last?.type === 'tool' ? [...prev.slice(0, -1), { ...last, done: true }] : prev; }); }
            else if (currentEvent === 'preview') { setGenPreview(chunk); const initFiles = {}; (chunk.files || []).forEach(f => { initFiles[f.path] = f.content; }); setEditedFiles(initFiles); if (chunk.files?.length > 0) setEditingFile(chunk.files[0].path); }
            else if (currentEvent === 'error') { setGenLogs(prev => [...prev, { type: 'error', text: chunk }]); }
            currentEvent = 'text';
          }
        }
      }
    } catch (e) { setGenLogs(prev => [...prev, { type: 'error', text: e.message }]); }
    finally { setGenLoading(false); }
  }, [genDesc, genLoading]);

  const handleConfirm = async () => {
    if (!genPreview) return;
    setGenLoading(true);
    try {
      const r = await fetch('/api/skills/generate/confirm', { method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ tmpDir: genPreview.tmpDir, skillName: genPreview.skillName, files: editedFiles }) });
      const data = await r.json();
      if (!r.ok) { alert(data.error || '提交失败'); return; }
      setSkills(prev => { const exists = prev.find(s => s.id === data.id); return exists ? prev.map(s => s.id === data.id ? data : s) : [data, ...prev]; });
      setGenPreview(null); setGenLogs([]); setGenDesc(''); setEditedFiles({}); setActiveTab('list');
    } catch (e) { alert('提交失败: ' + e.message); }
    finally { setGenLoading(false); }
  };

  const TABS = [
    { id: 'list', label: '已有技能', icon: Sparkles },
    { id: 'generate', label: 'AI 生成', icon: Wand2 },
    { id: 'upload', label: '上传压缩包', icon: Upload },
  ];

  return (
    <div className="max-w-5xl mx-auto">
      <div className="relative overflow-hidden rounded-2xl mb-6 p-6 surface-grad">
        <div className="absolute -right-20 -top-20 w-64 h-64 rounded-full bg-gradient-to-br from-[color:var(--accent)]/30 to-transparent blur-3xl pointer-events-none" />
        <div className="relative flex items-center gap-4">
          <div className="w-14 h-14 rounded-2xl bg-gradient-to-br from-[color:var(--accent)] to-[#5e8bff] text-white flex items-center justify-center shadow-glow">
            <Sparkles size={26} />
          </div>
          <div className="flex-1">
            <div className="text-2xl font-semibold tracking-tight text-gradient">技能管理</div>
            <div className="text-sm text-[color:var(--text-soft)]">导入、生成、安装和管理本地技能</div>
          </div>
        </div>
      </div>

      <div className="flex gap-1 p-1 bg-[color:var(--bg-soft)] rounded-lg mb-6">
        {TABS.map((t) => {
          const Icon = t.icon;
          return (
            <button key={t.id} onClick={() => setActiveTab(t.id)} className={cn(
              'flex-1 flex items-center justify-center gap-2 px-3 py-2 text-sm rounded-md transition',
              activeTab === t.id ? 'bg-[color:var(--bg-elev)] shadow-soft text-[color:var(--accent)] font-medium' : 'text-[color:var(--text-soft)] hover:text-[color:var(--text)]'
            )}>
              <Icon size={14} /> {t.label}
            </button>
          );
        })}
      </div>

      {activeTab === 'list' && (
        <div>
          {loading ? (
            <div className="py-20 text-center text-[color:var(--text-faint)]">
              <Loader2 size={24} className="animate-spin mx-auto mb-3" />加载中...
            </div>
          ) : skills.length === 0 ? (
            <div className="py-20 text-center">
              <Sparkles size={40} className="mx-auto mb-3 text-[color:var(--accent)] opacity-50" />
              <p className="text-[color:var(--text-soft)]">还没有技能，去 AI 生成或上传一个吧</p>
            </div>
          ) : (
            <div className="space-y-3">
              <AnimatePresence>
                {skills.map(skill => (
                  <motion.div key={skill.id} layout initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, scale: 0.95 }}>
                    <SkillCard skill={skill} onInstall={handleInstall} onUninstall={handleUninstall} onDelete={handleDelete} />
                  </motion.div>
                ))}
              </AnimatePresence>
            </div>
          )}
        </div>
      )}

      {activeTab === 'generate' && (
        <div className="max-w-3xl">
          {!genPreview ? (
            <>
              <div className="mb-5">
                <div className="text-sm font-medium mb-2">描述你想要的技能功能</div>
                <textarea
                  className="w-full min-h-[140px] rounded-lg px-4 py-3 bg-[color:var(--bg-elev)] border border-[color:var(--line)] text-[color:var(--text)] placeholder:text-[color:var(--text-faint)] focus:outline-none focus:ring-2 focus:ring-[color:var(--accent)]/30 focus:border-[color:var(--accent)] resize-y text-sm"
                  value={genDesc} onChange={e => setGenDesc(e.target.value)} disabled={genLoading}
                  placeholder={`例如：\n• 帮我创建一个 MySQL 数据库运维 skill\n• 创建一个 Git 工作流 skill\n• 我需要一个 Python 代码质量检查 skill`}
                />
                <Button className="mt-3" onClick={handleGenerate} disabled={genLoading || !genDesc.trim()}>
                  {genLoading ? <><Loader2 size={14} className="animate-spin" />生成中...</> : <><Wand2 size={14} />开始生成</>}
                </Button>
              </div>
              {genLogs.length > 0 && (
                <Card className="max-h-[300px] overflow-y-auto scrollable" ref={logRef}>
                  <div className="text-[10px] uppercase tracking-wide text-[color:var(--text-faint)] mb-2 font-medium">生成过程</div>
                  {genLogs.map((log, i) => (
                    <div key={i} className={cn('text-sm leading-relaxed', log.type === 'error' && 'text-red-500', log.type === 'tool' && 'flex items-center gap-2 text-xs text-[color:var(--text-faint)]')}>
                      {log.type === 'tool' && <span className={cn('w-1.5 h-1.5 rounded-full', log.done ? 'bg-emerald-500' : 'bg-[color:var(--accent)] animate-pulse')} />}
                      {log.text}
                    </div>
                  ))}
                  {genLoading && <span className="text-[color:var(--accent)] animate-pulse">▋</span>}
                </Card>
              )}
            </>
          ) : (
            <div className="space-y-4">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <div className="font-semibold">预览生成的技能: <code className="text-[color:var(--accent)] bg-[color:var(--accent-soft)] px-1.5 py-0.5 rounded text-sm">{genPreview.skillName}</code></div>
                  <div className="text-sm text-[color:var(--text-soft)] mt-1">你可以编辑文件内容，确认无误后点击「保存并发布」</div>
                </div>
                <div className="flex gap-2">
                  <Button variant="outline" onClick={() => { setGenPreview(null); setGenLogs([]); setEditedFiles({}); }} disabled={genLoading}>重新生成</Button>
                  <Button onClick={handleConfirm} disabled={genLoading}>
                    {genLoading ? <><Loader2 size={14} className="animate-spin" />保存中...</> : <><CheckCircle2 size={14} />保存并发布</>}
                  </Button>
                </div>
              </div>
              <div className="flex gap-0 h-[calc(100vh-340px)] min-h-[400px] surface overflow-hidden">
                <div className="w-[220px] shrink-0 border-r border-[color:var(--line)] py-3 overflow-y-auto scrollable">
                  <div className="text-[10px] uppercase tracking-wide text-[color:var(--text-faint)] px-3 pb-2 font-medium">文件结构</div>
                  {genPreview.files.map(f => {
                    const Icon = getFileIcon(f.path);
                    return (
                      <button key={f.path} onClick={() => setEditingFile(f.path)} className={cn(
                        'w-full flex items-center gap-2 px-3 py-1.5 text-xs transition text-left',
                        editingFile === f.path ? 'bg-[color:var(--accent-soft)] text-[color:var(--accent)]' : 'text-[color:var(--text-soft)] hover:bg-[color:var(--bg-soft)]'
                      )}>
                        <Icon size={12} /> <span className="truncate">{f.path}</span>
                      </button>
                    );
                  })}
                </div>
                <div className="flex-1 flex flex-col overflow-hidden">
                  {editingFile && (
                    <>
                      <div className="px-3 py-2 text-xs text-[color:var(--text-faint)] border-b border-[color:var(--line)] bg-[color:var(--bg-soft)] font-mono">{editingFile}</div>
                      {editingFile.endsWith('.md') ? (
                        <div className="flex-1 flex overflow-hidden">
                          <textarea className="flex-1 border-r border-[color:var(--line)] bg-transparent text-sm font-mono p-3 resize-none outline-none text-[color:var(--text)]" value={editedFiles[editingFile] || ''} onChange={e => setEditedFiles(prev => ({ ...prev, [editingFile]: e.target.value }))} />
                          <div className="flex-1 p-4 overflow-y-auto scrollable text-sm text-[color:var(--text-soft)] md-block">
                            <ReactMarkdown remarkPlugins={[remarkGfm]}>{editedFiles[editingFile] || ''}</ReactMarkdown>
                          </div>
                        </div>
                      ) : (
                        <textarea className="flex-1 bg-transparent text-sm font-mono p-3 resize-none outline-none text-[color:var(--text)]" value={editedFiles[editingFile] || ''} onChange={e => setEditedFiles(prev => ({ ...prev, [editingFile]: e.target.value }))} />
                      )}
                    </>
                  )}
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {activeTab === 'upload' && (
        <div className="max-w-xl">
          <Card className="mb-5">
            <div className="font-medium mb-2">Skill 压缩包结构要求</div>
            <pre className="bg-[color:var(--bg-soft)] rounded-lg p-3 text-xs font-mono text-[color:var(--text-soft)] leading-relaxed">{`<skill-name>/
├── SKILL.md          # 【必需】包含 frontmatter 和指令
├── scripts/          # 【可选】辅助脚本
├── references/       # 【可选】参考文档
└── assets/           # 【可选】资源文件`}</pre>
            <div className="text-xs text-[color:var(--text-faint)] mt-2">SKILL.md 必须包含 <code className="text-[color:var(--accent)]">name</code> 和 <code className="text-[color:var(--accent)]">description</code> frontmatter</div>
          </Card>

          <div
            className={cn(
              'border-2 border-dashed rounded-xl p-12 text-center cursor-pointer transition-all',
              uploadDragging ? 'border-[color:var(--accent)] bg-[color:var(--accent-soft)] shadow-[0_0_20px_var(--accent-glow)]' : 'border-[color:var(--line)] bg-[color:var(--bg-elev)] hover:border-[color:var(--accent)]',
              uploadLoading && 'cursor-default'
            )}
            onDragOver={e => { e.preventDefault(); setUploadDragging(true); }}
            onDragLeave={() => setUploadDragging(false)}
            onDrop={handleDrop}
            onClick={() => !uploadLoading && fileInputRef.current?.click()}
          >
            <input ref={fileInputRef} type="file" accept=".zip" multiple style={{ display: 'none' }} onChange={e => handleUploadFiles(e.target.files)} />
            {uploadLoading ? (
              <div className="flex items-center justify-center gap-3 text-[color:var(--text-soft)]">
                <Loader2 size={24} className="animate-spin" /> 上传中...
              </div>
            ) : (
              <>
                <Upload size={32} className="mx-auto mb-3 text-[color:var(--text-faint)]" />
                <div className="text-sm text-[color:var(--text-soft)]">拖拽一个或多个 .zip 文件到此处，或点击选择</div>
                <div className="text-xs text-[color:var(--text-faint)] mt-1">支持批量上传 · 仅支持 .zip 格式</div>
              </>
            )}
          </div>

          {uploadError && (
            <div className="mt-3 px-4 py-3 rounded-lg bg-red-500/10 border border-red-500/30 text-red-500 text-sm">{uploadError}</div>
          )}

          {uploadResults.length > 0 && (
            <Card className="mt-4 !p-0 overflow-hidden">
              <div className="px-4 py-2.5 bg-[color:var(--bg-soft)] border-b border-[color:var(--line)] text-xs font-medium text-[color:var(--text-soft)]">
                上传结果（{uploadResults.filter(r => r.success).length}/{uploadResults.length} 成功）
              </div>
              {uploadResults.map((r, i) => (
                <div key={i} className={cn('flex items-center gap-2 px-4 py-2.5 text-sm border-b border-[color:var(--line)] last:border-0', r.success ? 'bg-emerald-500/5' : 'bg-red-500/5')}>
                  {r.success ? <CheckCircle2 size={14} className="text-emerald-500" /> : <AlertCircle size={14} className="text-red-500" />}
                  <span className="font-mono text-xs flex-1 truncate">{r.filename}</span>
                  {r.error && <Badge tone="danger">{r.error}</Badge>}
                  {r.success && r.skill && <Badge tone="success">{r.skill.name}</Badge>}
                </div>
              ))}
              {uploadResults.every(r => r.success) && (
                <button onClick={() => { setUploadResults([]); setActiveTab('list'); }} className="w-full px-4 py-2.5 text-sm text-[color:var(--accent)] hover:bg-[color:var(--accent-soft)] transition">
                  查看已上传的技能 →
                </button>
              )}
            </Card>
          )}
        </div>
      )}
    </div>
  );
}

function SkillCard({ skill, onInstall, onUninstall, onDelete }) {
  const [expanded, setExpanded] = useState(false);
  return (
    <Card className={cn('transition-all hover:shadow-glow hover:-translate-y-0.5 group', skill.installed && 'border-emerald-500/30')}>
      <div className="flex items-start gap-3">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <code className="text-sm font-semibold text-[color:var(--accent)]">{skill.name}</code>
            {skill.installed && <Badge tone="success">已安装</Badge>}
          </div>
          <div className="text-sm text-[color:var(--text-soft)] line-clamp-2">{skill.description || '暂无描述'}</div>
        </div>
        <div className="flex gap-1.5 shrink-0">
          {skill.installed ? (
            <Button size="sm" variant="outline" onClick={() => onUninstall(skill)} disabled={skill._loading}>
              {skill._loading ? '...' : '卸载'}
            </Button>
          ) : (
            <Button size="sm" onClick={() => onInstall(skill)} disabled={skill._loading}>
              {skill._loading ? <Loader2 size={12} className="animate-spin" /> : <Download size={12} />}
              {skill._loading ? '...' : '安装'}
            </Button>
          )}
          <Button size="sm" variant="ghost" onClick={() => onDelete(skill)}><Trash2 size={14} /></Button>
        </div>
      </div>
      <div className="flex items-center gap-2 mt-3 text-xs text-[color:var(--text-faint)]">
        <span>创建者: {skill.created_by || '-'}</span>
        <span>·</span>
        <span>{new Date(skill.created_at).toLocaleDateString('zh-CN')}</span>
        <button onClick={() => setExpanded(o => !o)} className="ml-auto flex items-center gap-1 hover:text-[color:var(--text-soft)] transition">
          {expanded ? <><ChevronDown size={12} />收起</> : <><ChevronRight size={12} />详情</>}
        </button>
      </div>
      {expanded && (
        <div className="mt-3 pt-3 border-t border-[color:var(--line)]">
          <div className="flex items-center gap-2 text-xs">
            <span className="text-[color:var(--text-faint)]">OSS 路径</span>
            <code className="text-[color:var(--text-soft)] bg-[color:var(--bg-soft)] px-1.5 py-0.5 rounded text-[11px]">{skill.oss_key}</code>
          </div>
        </div>
      )}
    </Card>
  );
}
