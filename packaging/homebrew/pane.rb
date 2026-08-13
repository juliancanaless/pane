class Pane < Formula
  desc "Shared local memory and coordination for concurrent coding agents"
  homepage "https://github.com/juliancanaless/pane"
  version "0.1.5"
  license "MIT"
  head "https://github.com/juliancanaless/pane.git", branch: "main"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.5/pane-v0.1.5-darwin-arm64.tar.gz"
      sha256 "82612f159b6ca50a726bd57aacea831f96252b01319c3270db6d4f06dd647a6f"
    else
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.5/pane-v0.1.5-darwin-amd64.tar.gz"
      sha256 "dcde3caa1b23cd9818cb9ccf9d79c92fbc6e30d988ecaf3cfcce857c5d847ef6"
    end
  end

  depends_on "go" => :build if build.head?
  depends_on "rust" => :build if build.head?

  def install
    if build.head?
      ENV.prepend_path "PATH", Formula["rust"].opt_bin
      system "make", "build"
      bin.install "bin/pane"
      bin.install "bin/pane-analyze"
    else
      bin.install "pane"
      bin.install "pane-analyze"
    end
  end

  test do
    assert_match "Pane gives concurrent coding agents", shell_output("#{bin}/pane help")
    assert_match "platform:", shell_output("#{bin}/pane doctor")
  end
end
