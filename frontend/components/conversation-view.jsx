function renderTextWithRuby(text, rubyTokens) {
  if (!text || !rubyTokens?.length) {
    return text;
  }

  const content = [];
  let cursor = 0;

  rubyTokens.forEach((token, index) => {
    if (!token?.surface || !token?.reading) {
      return;
    }

    const position = text.indexOf(token.surface, cursor);
    if (position === -1) {
      return;
    }

    if (position > cursor) {
      content.push(text.slice(cursor, position));
    }

    content.push(
      <ruby key={`${token.surface}-${token.reading}-${index}`}>
        {token.surface}
        <rt>{token.reading}</rt>
      </ruby>,
    );

    cursor = position + token.surface.length;
  });

  if (cursor < text.length) {
    content.push(text.slice(cursor));
  }

  return content;
}

export default function ConversationView({ conversation }) {
  const turns = conversation?.turns ?? [];

  if (!turns.length) {
    return <p className="conversation-empty">这个项目目前还没有可展示的对话内容。</p>;
  }

  return (
    <div className="conversation-list">
      {turns.map((turn, index) => (
        <article className="conversation-turn" key={turn.turn_id || index}>
          <div className="turn-header">
            <div className="speaker-badge">
              <span className="speaker-dot" />
              <span>{turn.speaker_name || turn.speaker || "Unknown speaker"}</span>
            </div>
            <span className="turn-index">Turn {String(index + 1).padStart(2, "0")}</span>
          </div>

          <div className="segment-list">
            {(turn.segments ?? []).map((segment, segmentIndex) => (
              <div className="segment-card" key={segment.segment_id || segmentIndex}>
                <p className="display-text">{renderTextWithRuby(segment.display_text, segment.ruby)}</p>
                {segment.english ? <p className="translation">{segment.english}</p> : null}
                {segment.ruby?.length ? (
                  <div className="ruby-list">
                    {segment.ruby.map((token, rubyIndex) => (
                      <span
                        className="ruby-token"
                        key={`${token.surface}-${token.reading}-${rubyIndex}`}
                      >
                        {token.surface} · {token.reading}
                      </span>
                    ))}
                  </div>
                ) : null}
              </div>
            ))}
          </div>
        </article>
      ))}
    </div>
  );
}
