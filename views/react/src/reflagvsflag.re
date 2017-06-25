type dom;

external dom : dom = "document" [@@bs.val];

module Cookies = {
  external get_all_cookies : dom => string = "cookie" [@@bs.get];
  external set_cookie : dom => string => unit = "cookie" [@@bs.set];
  exception NotFound unit;
  let getCookie name => {
    /* this algorithm based on an answer by StackOverflow user "kirlich" to the question:
       https://stackoverflow.com/questions/10730362/get-cookie-by-name */
    let all = "; " ^ get_all_cookies dom;
    let regex = Js.Re.fromString ("; " ^ name ^ "=(.*)[$;]");
    let result = Js.Re.exec all regex;
    switch result {
    | None => None
    | Some res =>
      let matches = Js.Re.matches res;
      Some matches.(1)
    }
  };
  let updateSelectedTags (tags: list Tags.tag) => {
    let cookie =
      switch tags {
      | [] => ""
      | [x] => x.name
      | [hd, ...tl] => List.fold_left (fun acc (tag: Tags.tag) => acc ^ "," ^ tag.name) hd.name tl
      };
    set_cookie dom ("selected_tags=" ^ cookie)
  };
  let getSelectedTags () => {
    let cookie = getCookie "selected_tags";
    switch cookie {
    | Some cookie => Array.to_list (Js.String.split cookie ",")
    | None => []
    }
  };
  let getAllTags () => {
    let cookie = getCookie "all_tags";
    switch cookie {
    | Some cookie => Array.to_list (Js.String.split cookie ",")
    | None => []
    }
  };
};

external getById : dom => string => Dom.element = "getElementById" [@@bs.send];

let tags: list Tags.tag = {
  let all = Cookies.getAllTags ();
  Js.log2 "Cookies.getAllTags() == " all;
  let sels = Cookies.getSelectedTags ();
  let sels =
    if (List.length sels == 0) {
      let tags = ["Modern"];
      Js.log "No tags given, defaulting to [Modern]";
      Cookies.updateSelectedTags (List.map Tags.of_string tags);
      tags
    } else {
      sels
    };
  Js.log2 "sels from Reflagvsflag.tags = " sels;
  List.map
    (fun (tag: string) => ({name: tag, selected: List.exists ((==) tag) sels}: Tags.tag)) all
};

Js.log tags;

let rfvfTagSelectorContainer = getById dom "rfvfTagSelector";

Js.log rfvfTagSelectorContainer;

ReactDOMRe.render
  <TagSelector updateSelected=Cookies.updateSelectedTags tags /> rfvfTagSelectorContainer;
