type dom;

external dom : dom = "document" [@@bs.val];

module Cookies = {
  external get_all_cookies : dom => string = "cookie" [@@bs.get];
  external set_cookie : dom => string => unit = "cookie" [@@bs.set];
  exception NotFound unit;
  let getCookie name => {
    /* this algorithm based on an answer by StackOverflow user "kirlich" to the question:
       https://stackoverflow.com/questions/10730362/get-cookie-by-name */
    let all = "; " ^ get_all_cookies dom ^ ";";
    let regex = Js.Re.fromString ("; " ^ name ^ "=([^;]*);");
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
    | Some cookie => Array.to_list (Js.String.split "," cookie)
    | None => []
    }
  };
  let getAllTags () => {
    let cookie = getCookie "all_tags";
    switch cookie {
    | Some cookie => Array.to_list (Js.String.split "," cookie)
    | None => []
    }
  };
};

external getById : dom => string => Dom.element = "getElementById" [@@bs.send];

module StringSet = Set.Make String;

let tags: list Tags.tag = {
  let all = StringSet.of_list (Cookies.getAllTags ());
  Js.log2 "Cookies.getAllTags() from Reflagvsflag.tags = " all;
  let sels = {
    let sels = Cookies.getSelectedTags ();
    Js.log2 "Cookies.getSelectedTags() from Reflagvsflag.tags.sels = " sels;
    switch sels {
    | [] =>
      let sels = ["Modern"];
      Js.log "No tags given, defaulting to [Modern]";
      Cookies.updateSelectedTags (List.map Tags.of_string sels);
      StringSet.of_list sels
    | sels => StringSet.of_list sels
    }
  };
  Js.log2 "sels from Reflagvsflag.tags = " sels;
  let all = StringSet.union all sels;
  Js.log2 "all from Reflagvsflag.tags after union with sels = " all;
  let all = StringSet.elements all;
  let sels = StringSet.elements sels;
  Js.log2 "all from Reflagvsflag.tags after recovering elements from set = " all;
  Js.log2 "sels from Reflagvsflag.tags after recovering elements from set = " sels;
  List.map
    (fun (tag: string) => ({name: tag, selected: List.exists ((==) tag) sels}: Tags.tag)) all
};

Js.log tags;

let rfvfTagSelectorContainer = getById dom "rfvfTagSelector";

Js.log rfvfTagSelectorContainer;

ReactDOMRe.render
  <TagSelector updateSelected=Cookies.updateSelectedTags tags /> rfvfTagSelectorContainer;
