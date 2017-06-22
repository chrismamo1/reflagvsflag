let component = ReasonReact.statelessComponent "TagSelectorButton";

let make tag::(tag: Tags.tag) ::handler _children => {
  ...component,
  render: fun () _self => {
    let className = {
      let first = "pure-u-lg-11-24 pure-u-md-1 pure-u-1 pure-button rfvf-tag-selector-item";
      if tag.selected {
        first ^ " pure-button-secondary"
      } else {
        first
      }
    };
    <button _type="button" value=tag.name onClick=handler className>
      (ReasonReact.stringToElement tag.name)
    </button>
  }
};
