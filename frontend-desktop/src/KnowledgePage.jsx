import { useState, useRef, useEffect } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

const CATEGORY_LABELS = { docs: '文档', qa: '问答', data: '数据' };
const CATEGORY_ICONS = { docs: '📄', qa: '💬', data: '📊' };
const ALLOWED_EXTS = ['.md', '.txt', '.csv', '.tsv', '.json'];

function formatSize(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / 1024 / 1024).toFixed(1) + ' MB';
}

function parseTags(tags) {
  if (!tags || tags === '[]') return [];
  try { return JSON.parse(tags); } catch { return []; }
}

function getExt(name) {
  const m = name.match(/\.[^.]+$/);
  return m ? m[0].toLowerCase() : '';
}

// ─── 知识库条目卡片 ───────────────────────────────────────────────
function KnowledgeCard({ item, onDelete, onPreview }) {
  const tags = parseTags(item.tags);
  return (
    <div className="kb-card">
      <div className="kb-card-header">
        <span className="kb-card-icon">{CATEGORY_ICONS[item.category] || '📄'}</span>
        <div className="kb-card-info">
          <div className="kb-card-title">{item.title}</div>
          <div className="kb-card-meta">
            <span className="kb-card-category">{CATEGORY_LABELS[item.category] || item.category}</span>
            <span className="kb-card-size">{formatSize(item.size)}</span>
            <span className="kb-card-date">
              {new Date(item.created_at).toLocaleDateString('zh-CN')}
            </span>
          </div>
        </div>
        <div className="kb-card-actions">
          <button className="kb-btn preview" onClick={() => onPreview(item)}>预览</button>
          <button className="kb-btn delete" onClick={() => onDelete(item)}>删除</button>
        </div>
      </div>
      {item.summary && (
        <div className="kb-card-summary">{item.summary}</div>
      )}
      {tags.length > 0 && (
        <div className="kb-card-tags">
          {tags.map((t, i) => <span key={i} className="kb-tag">{t}</span>)}
        </div>
      )}
    </div>
  );
}

// ─── 预览弹窗 ────────────────────────────────────────────────────
function PreviewModal({ item, onClose }) {
  const [content, setContent] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch(`/api/knowledge/${item.id}/preview`, { credentials: 'include' })
      .then(r => r.json())
      .then(data => { setContent(data.content || ''); setLoading(false); })
      .catch(() => { setContent('加载失败'); setLoading(false); });
  }, [item.id]);

  return (
    <div className="kb-modal-overlay" onClick={onClose}>
      <div className="kb-modal" onClick={e => e.stopPropagation()}>
        <div className="kb-modal-header">
          <span className="kb-modal-title">{item.title}</span>
          <button className="kb-modal-close" onClick={onClose}>✕</button>
        </div>
        <div className="kb-modal-body">
          {loading ? (
            <div className="kb-modal-loading">加载中...</div>
          ) : (
            <div className="kb-preview-content">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// ─── 批量上传队列项 ──────────────────────────────────────────────
// status: 'pending' | 'uploading' | 'done' | 'error'
function QueueItem({ item, onRemove }) {
  const statusIcon = {
    pending: '⏳',
    uploading: '⚡',
    done: '✅',
    error: '❌',
  }[item.status] || '⏳';

  return (
    <div className={`kb-queue-item kb-queue-${item.status}`}>
      <span className="kb-queue-icon">{statusIcon}</span>
      <span className="kb-queue-name">{item.file.name}</span>
      <span className="kb-queue-size">{formatSize(item.file.size)}</span>
      {item.status === 'error' && (
        <span className="kb-queue-error">{item.error}</span>
      )}
      {item.status === 'pending' && (
        <button className="kb-queue-remove" onClick={() => onRemove(item.id)}>✕</button>
      )}
    </div>
  );
}

// ─── 主页面 ──────────────────────────────────────────────────────
export default function KnowledgePage({ onBack }) {
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState('list');
  const [previewItem, setPreviewItem] = useState(null);

  // 批量上传状态
  const [dragging, setDragging] = useState(false);
  const [uploadCategory, setUploadCategory] = useState('docs');
  const [uploadTags, setUploadTags] = useState('');
  const [queue, setQueue] = useState([]); // [{id, file, status, error}]
  const [uploading, setUploading] = useState(false);
  const [uploadDone, setUploadDone] = useState(false);
  const fileInputRef = useRef(null);
  const queueIdRef = useRef(0);

  const fetchItems = () => {
    setLoading(true);
    fetch('/api/knowledge', { credentials: 'include' })
      .then(r => r.json())
      .then(data => { setItems(Array.isArray(data) ? data : []); setLoading(false); })
      .catch(() => setLoading(false));
  };

  useEffect(() => { fetchItems(); }, []);

  const handleDelete = async (item) => {
    if (!window.confirm(`确定删除「${item.title}」？`)) return;
    await fetch(`/api/knowledge/${item.id}`, { method: 'DELETE', credentials: 'include' });
    fetchItems();
  };

  // 将 File 列表加入队列（过滤不支持的格式和超大文件）
  const addFilesToQueue = (files) => {
    const newItems = [];
    for (const file of files) {
      const ext = getExt(file.name);
      if (!ALLOWED_EXTS.includes(ext)) continue;
      if (file.size > 10 * 1024 * 1024) continue;
      queueIdRef.current += 1;
      newItems.push({ id: queueIdRef.current, file, status: 'pending', error: '' });
    }
    setQueue(prev => [...prev, ...newItems]);
    setUploadDone(false);
  };

  const handleDrop = (e) => {
    e.preventDefault();
    setDragging(false);
    addFilesToQueue(Array.from(e.dataTransfer.files));
  };

  const handleFileInput = (e) => {
    addFilesToQueue(Array.from(e.target.files));
    e.target.value = '';
  };

  const removeFromQueue = (id) => {
    setQueue(prev => prev.filter(item => item.id !== id));
  };

  const clearQueue = () => {
    setQueue([]);
    setUploadDone(false);
  };

  // 逐个上传队列中 pending 的文件
  const handleUploadAll = async () => {
    const pending = queue.filter(item => item.status === 'pending');
    if (pending.length === 0) return;

    setUploading(true);
    const tagsArr = uploadTags.split(',').map(t => t.trim()).filter(Boolean);

    for (const item of pending) {
      // 标记为上传中
      setQueue(prev => prev.map(q => q.id === item.id ? { ...q, status: 'uploading' } : q));

      const form = new FormData();
      form.append('file', item.file);
      form.append('title', item.file.name.replace(/\.[^.]+$/, ''));
      form.append('category', uploadCategory);
      form.append('tags', JSON.stringify(tagsArr));

      try {
        const res = await fetch('/api/knowledge', {
          method: 'POST',
          credentials: 'include',
          body: form,
        });
        const data = await res.json();
        if (!res.ok) {
          setQueue(prev => prev.map(q => q.id === item.id ? { ...q, status: 'error', error: data.error || '上传失败' } : q));
        } else {
          setQueue(prev => prev.map(q => q.id === item.id ? { ...q, status: 'done' } : q));
        }
      } catch (err) {
        setQueue(prev => prev.map(q => q.id === item.id ? { ...q, status: 'error', error: err.message } : q));
      }
    }

    setUploading(false);
    setUploadDone(true);
    fetchItems();
  };

  const pendingCount = queue.filter(q => q.status === 'pending').length;
  const doneCount = queue.filter(q => q.status === 'done').length;
  const errorCount = queue.filter(q => q.status === 'error').length;

  // 按分类分组
  const grouped = { docs: [], qa: [], data: [] };
  items.forEach(item => {
    const cat = item.category || 'docs';
    if (!grouped[cat]) grouped[cat] = [];
    grouped[cat].push(item);
  });

  return (
    <div className="skills-page">
      {/* 顶部导航 */}
      <div className="skills-header">
        <button className="skills-back-btn" onClick={onBack}>← 返回</button>
        <h2 className="skills-title">知识库</h2>
        <div className="skills-tabs">
          <button
            className={`skills-tab${activeTab === 'list' ? ' active' : ''}`}
            onClick={() => setActiveTab('list')}
          >
            文件列表 {items.length > 0 && <span className="tab-count">{items.length}</span>}
          </button>
          <button
            className={`skills-tab${activeTab === 'upload' ? ' active' : ''}`}
            onClick={() => setActiveTab('upload')}
          >
            上传文件
          </button>
        </div>
      </div>

      {/* 文件列表 */}
      {activeTab === 'list' && (
        <div className="skills-body">
          {loading ? (
            <div className="skills-loading">加载中...</div>
          ) : items.length === 0 ? (
            <div className="skills-empty">
              <div className="skills-empty-icon">📚</div>
              <div className="skills-empty-text">知识库为空</div>
              <div className="skills-empty-hint">上传 .md .txt .csv 等文件，灵犀会在回答时自动参考</div>
              <button className="skill-btn install" onClick={() => setActiveTab('upload')}>上传文件</button>
            </div>
          ) : (
            <div className="kb-list">
              {Object.entries(grouped).map(([cat, catItems]) =>
                catItems.length === 0 ? null : (
                  <div key={cat} className="kb-group">
                    <div className="kb-group-title">
                      {CATEGORY_ICONS[cat]} {CATEGORY_LABELS[cat]}
                      <span className="kb-group-count">{catItems.length}</span>
                    </div>
                    {catItems.map(item => (
                      <KnowledgeCard
                        key={item.id}
                        item={item}
                        onDelete={handleDelete}
                        onPreview={setPreviewItem}
                      />
                    ))}
                  </div>
                )
              )}
            </div>
          )}
        </div>
      )}

      {/* 批量上传 */}
      {activeTab === 'upload' && (
        <div className="skills-body kb-upload-body">
          {/* 可滚动内容区 */}
          <div className="kb-upload-scroll">
            <div className="upload-guide">
              <div className="upload-guide-title">批量上传知识库文件</div>
              <p className="upload-guide-desc">
                支持 <code>.md</code> <code>.txt</code> <code>.csv</code> <code>.tsv</code> <code>.json</code>，
                单文件不超过 10MB，可一次选择多个文件。上传后灵犀会在回答时自动检索参考。
              </p>
            </div>

            {/* 拖拽上传区 */}
            <div
              className={`upload-zone${dragging ? ' dragging' : ''}`}
              onDragOver={e => { e.preventDefault(); setDragging(true); }}
              onDragLeave={() => setDragging(false)}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
            >
              <input
                ref={fileInputRef}
                type="file"
                accept=".md,.txt,.csv,.tsv,.json"
                multiple
                style={{ display: 'none' }}
                onChange={handleFileInput}
              />
              <div className="upload-icon">📂</div>
              <div className="upload-text">拖拽文件到此处，或点击选择文件</div>
              <div className="upload-hint">支持多选，.md .txt .csv .tsv .json，每个文件最大 10MB</div>
            </div>

            {/* 公共设置 */}
            <div className="kb-form">
              <div className="kb-form-row">
                <label className="kb-form-label">分类</label>
                <select
                  className="kb-form-select"
                  value={uploadCategory}
                  onChange={e => setUploadCategory(e.target.value)}
                >
                  <option value="docs">📄 文档</option>
                  <option value="qa">💬 问答</option>
                  <option value="data">📊 数据</option>
                </select>
              </div>
              <div className="kb-form-row">
                <label className="kb-form-label">标签</label>
                <input
                  className="kb-form-input"
                  type="text"
                  placeholder="多个标签用逗号分隔（可选）"
                  value={uploadTags}
                  onChange={e => setUploadTags(e.target.value)}
                />
              </div>
            </div>

            {/* 上传队列 */}
            {queue.length > 0 && (
              <div className="kb-queue">
                <div className="kb-queue-header">
                  <span className="kb-queue-title">
                    待上传 {pendingCount} 个
                    {doneCount > 0 && <span className="kb-queue-done-count"> · 已完成 {doneCount}</span>}
                    {errorCount > 0 && <span className="kb-queue-err-count"> · 失败 {errorCount}</span>}
                  </span>
                  {!uploading && (
                    <button className="kb-queue-clear" onClick={clearQueue}>清空</button>
                  )}
                </div>
                <div className="kb-queue-list">
                  {queue.map(item => (
                    <QueueItem key={item.id} item={item} onRemove={removeFromQueue} />
                  ))}
                </div>
              </div>
            )}

            {uploadDone && errorCount === 0 && (
              <div className="kb-upload-success">
                ✅ 全部 {doneCount} 个文件上传成功！
                <button className="kb-upload-view" onClick={() => setActiveTab('list')}>查看知识库</button>
              </div>
            )}
          </div>

          {/* 上传按钮固定在底部 */}
          <div className="kb-upload-footer">
            <button
              className="skill-btn install"
              style={{ width: '100%' }}
              onClick={handleUploadAll}
              disabled={uploading || pendingCount === 0}
            >
              {uploading ? `上传中... (${doneCount + errorCount}/${queue.length})` : `上传 ${pendingCount} 个文件`}
            </button>
          </div>
        </div>
      )}

      {/* 预览弹窗 */}
      {previewItem && (
        <PreviewModal item={previewItem} onClose={() => setPreviewItem(null)} />
      )}
    </div>
  );
}
