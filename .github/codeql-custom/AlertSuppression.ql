/**
 * @name Alert suppression
 * @description Generates information about alert suppressions.
 * @kind alert-suppression
 * @id go/alert-suppression
 *
 * 本文件是 github/codeql go/ql/src/AlertSuppression.ql 的随仓副本（上游同源，仅补齐
 * 依赖声明），因为高级设置的 queries 输入无法直接引用 registry 包内的单个查询，
 * 而 // codeql[...] 抑制注释必须先由本查询算出 suppressions[] 才会生效。
 */

private import codeql.util.suppression.AlertSuppression as AS
private import semmle.go.Comments as G

class SingleLineComment extends G::Comment {
  SingleLineComment() {
    // suppression comments must be single-line
    not this.getText().matches("%\n%")
  }
}

import AS::Make<G::Locatable, SingleLineComment>
