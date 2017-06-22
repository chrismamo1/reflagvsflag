let allAreSelected tags => not (List.exists (fun (tag: Tags.tag) => not tag.selected) tags);

let toggleOne (t: Tags.tag) tags =>
  List.map
    (
      fun (tag: Tags.tag) =>
        if (tag.name == t.name) {
          {...tag, selected: not tag.selected}
        } else {
          tag
        }
    )
    tags;

let component = ReasonReact.statefulComponent "TagSelector";

let make ::updateSelected ::tags _children => {
  let handleUpdate tag _event state _self => {
    let tags = toggleOne tag state;
    let selected = List.filter (fun (tag: Tags.tag) => tag.selected) tags;
    updateSelected selected;
    ReasonReact.Update tags
  };
  let selectAll _event state _self => {
    let newVal =
      List.map (fun (tag: Tags.tag) => {...tag, selected: not (allAreSelected state)}) state;
    updateSelected newVal;
    ReasonReact.Update newVal
  };
  {
    ...component,
    initialState: fun () => tags,
    render: fun state self => {
      let allSel = {
        let text =
          if (allAreSelected state) {
            "Deselect All"
          } else {
            "Select All"
          };
        <button
          _type="button"
          value=text
          onClick=(self.update selectAll)
          className="pure-u-1 pure-u-md-1 pure-u-lg-22-24 pure-button rfvf-tag-selector-item pure-button-primary">
          (ReasonReact.stringToElement text)
        </button>
      };
      let buttons =
        List.map
          (fun tag => <TagSelectorButton tag handler=(self.update (handleUpdate tag)) />) state;
      let contents = ReasonReact.arrayToElement (Array.of_list buttons);
      <div className="pure-button-group" role="group"> contents allSel </div>
    }
  }
};
