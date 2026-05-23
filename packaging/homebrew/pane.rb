class Pane < Formula
  desc "Shared local memory and coordination for concurrent coding agents"
  homepage "https://github.com/juliancanaless/pane"
  license "MIT"
  head "https://github.com/juliancanaless/pane.git", branch: "main"

  depends_on "go" => :build
  depends_on "rust" => :build

  def install
    system "make", "build"
    bin.install "bin/pane"
    bin.install "bin/pane-analyze"
  end

  test do
    assert_match "Pane gives concurrent coding agents", shell_output("#{bin}/pane help")
    assert_match "platform:", shell_output("#{bin}/pane doctor")
  end
end
