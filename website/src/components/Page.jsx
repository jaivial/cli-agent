export default function Page({ kicker, title, lead, children }) {
  return (
    <div className="page">
      <div className="container">
        <header className="page-header">
          {kicker ? <div className="kicker">{kicker}</div> : null}
          {title ? <h1 className="page-title">{title}</h1> : null}
          {lead ? <p className="page-lead">{lead}</p> : null}
        </header>
        <div className="page-body">{children}</div>
      </div>
    </div>
  )
}

